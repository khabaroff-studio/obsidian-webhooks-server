/**
 * polling_handler.ts - Polling handler for webhook events
 *
 * Provides a fallback mechanism to fetch events when SSE is unavailable
 * or as a complementary sync mechanism alongside SSE.
 */

import { requestUrl } from "obsidian";
import type { WebhookEvent } from "../types";

/**
 * Callback type for handling received events
 */
type EventCallback = (event: WebhookEvent) => Promise<void>;

/**
 * PollingHandler manages periodic polling of the webhook server for new events.
 * Used as a fallback when SSE is unavailable or to ensure sync in offline scenarios.
 */
export class PollingHandler {
	private serverUrl: string;
	private clientKey: string;
	private onEvent: EventCallback;
	private polling: boolean = false;
	private pollingTimer: ReturnType<typeof setInterval> | null = null;
	private defaultIntervalMs: number = 5000; // 5 seconds default

	/**
	 * Create a new PollingHandler
	 *
	 * @param serverUrl - Base URL of the webhook server
	 * @param clientKey - Client authentication key
	 * @param onEvent - Callback to invoke for each received event
	 */
	constructor(serverUrl: string, clientKey: string, onEvent: EventCallback) {
		this.serverUrl = serverUrl;
		this.clientKey = clientKey;
		this.onEvent = onEvent;
	}

	/**
	 * Perform a single poll request to fetch events
	 *
	 * Fetches unprocessed events from the server and invokes the callback for each.
	 * Errors are caught and logged but do not throw.
	 */
	async pollOnce(): Promise<void> {
		try {
			const url = `${this.serverUrl}/events/${this.clientKey}?poll=true`;
			const response = await requestUrl({ url, method: "GET", throw: false });

			if (response.status < 200 || response.status >= 300) {
				console.error(`Polling failed: ${response.status}`);
				return;
			}

			const events = response.json as WebhookEvent[];

			// Process each event through the callback
			for (const event of events) {
				try {
					await this.onEvent(event);
				} catch (error) {
					// Log but don't stop processing other events
					console.error(
						`Error processing event ${event.id}:`,
						error instanceof Error ? error.message : error
					);
				}
			}
		} catch (error) {
			// Network or parsing errors - log but don't throw
			console.error(
				"Polling error:",
				error instanceof Error ? error.message : error
			);
		}
	}

	/**
	 * Start continuous polling at the specified interval
	 *
	 * @param intervalMs - Polling interval in milliseconds (default: 5000ms)
	 */
	start(intervalMs?: number): void {
		// Prevent multiple polling loops
		if (this.polling) {
			return;
		}

		this.polling = true;
		const interval = intervalMs ?? this.defaultIntervalMs;

		// Poll immediately on start
		void this.pollOnce();

		// Set up recurring poll
		this.pollingTimer = setInterval(() => {
			void this.pollOnce();
		}, interval);
	}

	/**
	 * Stop continuous polling
	 *
	 * Safe to call multiple times or when not polling.
	 */
	stop(): void {
		this.polling = false;

		if (this.pollingTimer) {
			clearInterval(this.pollingTimer);
			this.pollingTimer = null;
		}
	}

	/**
	 * Check if polling is currently active
	 *
	 * @returns true if polling is running, false otherwise
	 */
	isPolling(): boolean {
		return this.polling;
	}
}
