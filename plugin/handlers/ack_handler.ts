/**
 * ack_handler.ts - ACK (Acknowledgment) handler for webhook events
 *
 * Handles sending acknowledgments to the webhook server after events are processed.
 * Implements retry logic with exponential backoff for reliability.
 *
 * Key features:
 * - Non-blocking error handling (failures don't break event processing)
 * - Retry logic for network failures and 5xx errors
 * - Idempotent (duplicate ACKs are handled gracefully)
 * - Configurable retry behavior
 * - Optional logging callbacks
 */

/**
 * Configuration options for ACKHandler
 */
export interface ACKHandlerOptions {
	/** Maximum number of retry attempts (default: 3) */
	maxRetries?: number;

	/** Initial retry delay in milliseconds (default: 1000) */
	retryDelayMs?: number;

	/** Callback called when ACK succeeds */
	onSuccess?: (eventId: string) => void;

	/** Callback called when ACK fails after all retries */
	onError?: (eventId: string, error: Error) => void;

	/** Callback called on each retry attempt */
	onRetry?: (eventId: string, attempt: number, error: Error) => void;
}

/**
 * Default configuration
 */
const DEFAULT_OPTIONS: Required<ACKHandlerOptions> = {
	maxRetries: 3,
	retryDelayMs: 1000,
	onSuccess: () => {},
	onError: () => {},
	onRetry: () => {},
};

/**
 * ACKHandler sends acknowledgments to the webhook server
 */
export class ACKHandler {
	private serverUrl: string;
	private clientKey: string;
	private options: Required<ACKHandlerOptions>;

	/**
	 * Create a new ACKHandler
	 *
	 * @param serverUrl - Base URL of the webhook server
	 * @param clientKey - Client key for authentication
	 * @param options - Optional configuration
	 */
	constructor(
		serverUrl: string,
		clientKey: string,
		options?: ACKHandlerOptions
	) {
		this.serverUrl = serverUrl;
		this.clientKey = clientKey;
		this.options = { ...DEFAULT_OPTIONS, ...options };
	}

	/**
	 * Acknowledge that an event has been processed
	 *
	 * @param eventId - The event ID to acknowledge
	 * @returns Promise<boolean> - true if ACK successful, false otherwise
	 * @throws Error if client key is not configured
	 */
	async acknowledgeEvent(eventId: string): Promise<boolean> {
		// Validate client key
		if (!this.clientKey || this.clientKey.trim() === "") {
			throw new Error("Cannot acknowledge event: no client key configured");
		}

		// Attempt ACK with retries
		let lastError: Error | null = null;

		for (let attempt = 0; attempt <= this.options.maxRetries; attempt++) {
			try {
				const success = await this.sendACK(eventId);

				if (success) {
					// ACK succeeded
					this.options.onSuccess(eventId);
					return true;
				} else {
					// HTTP error that we shouldn't retry (4xx except 409)
					return false;
				}
			} catch (error) {
				lastError = error instanceof Error ? error : new Error(String(error));

				// Check if we should retry
				if (attempt < this.options.maxRetries) {
					// Call onRetry callback
					this.options.onRetry(eventId, attempt + 1, lastError);

					// Wait before retrying with exponential backoff
					const delay = this.options.retryDelayMs * Math.pow(2, attempt);
					await this.sleep(delay);
				}
			}
		}

		// All retries exhausted
		if (lastError) {
			this.options.onError(eventId, lastError);
		}
		return false;
	}

	/**
	 * Send ACK to the server
	 *
	 * @param eventId - The event ID to acknowledge
	 * @returns Promise<boolean> - true if successful, false for non-retryable errors
	 * @throws Error for retryable errors (network failures, 5xx errors)
	 */
	private async sendACK(eventId: string): Promise<boolean> {
		const url = `${this.serverUrl}/ack/${this.clientKey}/${eventId}`;

		try {
			const response = await fetch(url, {
				method: "POST",
				headers: {
					"Content-Type": "application/json",
				},
			});

			if (response.ok) {
				// Success (200, 204, etc.)
				return true;
			}

			// Check for special cases
			if (response.status === 409) {
				// Conflict - event already processed (idempotent)
				return true;
			}

			// 5xx errors - retry
			if (response.status >= 500) {
				throw new Error(
					`Server error: ${response.status} ${response.statusText}`
				);
			}

			// 4xx errors (except 409) - don't retry
			return false;
		} catch (error) {
			// Network errors and other exceptions - retry
			throw error;
		}
	}

	/**
	 * Sleep for the specified duration
	 *
	 * @param ms - Milliseconds to sleep
	 */
	private sleep(ms: number): Promise<void> {
		return new Promise((resolve) => setTimeout(resolve, ms));
	}
}
