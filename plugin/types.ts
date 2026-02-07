/**
 * types.ts - TypeScript interface definitions for Obsidian Webhooks Selfhosted Plugin
 *
 * This file defines all the core types used throughout the plugin for webhook events,
 * settings, and connection state management.
 */

/**
 * WebhookEvent represents a single event received from the webhook server.
 * Events are delivered via SSE or polling and contain the data to be written to vault.
 */
export interface WebhookEvent {
	/** Unique identifier for the event (used for deduplication and ACK) */
	id: string;

	/** Path in the vault where content should be written (e.g., "inbox/note.md") */
	path: string;

	/** The actual content/data to write to the file */
	data?: string;

	/** ISO 8601 timestamp when the event was created on the server */
	created_at: string;

	/** Whether the event has been acknowledged (processed) by the client */
	processed?: boolean;
}

/**
 * WebhookSettings stores the user's plugin configuration.
 * Persisted in Obsidian's data.json for the plugin.
 */
export interface WebhookSettings {
	/** Base URL of the webhook server (e.g., "https://webhooks.example.com") */
	serverUrl: string;

	/** Client key for authentication (format: "cl_...") */
	clientKey: string;

	/** Whether to automatically connect to SSE on plugin load */
	autoConnect: boolean;

	/** Default write mode: append to end of file or overwrite */
	defaultMode: "append" | "overwrite";

	/** Type of newline to add between incoming notes (none, windows, unix) */
	newlineType: "none" | "windows" | "unix";

	/** Polling interval in seconds (used as fallback when SSE is unavailable) */
	pollingInterval: number;

	/** Whether to enable polling alongside SSE */
	enablePolling: boolean;

	/** Whether to show notifications for successful event processing */
	showNotifications: boolean;

	/** Whether to log debug information to console */
	enableDebugLogging: boolean;

	/** Maximum number of retry attempts for ACK sending */
	maxRetries?: number;

	/** Delay between retry attempts in milliseconds */
	retryDelayMs?: number;

	/** Automatically create parent folders if they don't exist */
	autoCreateFolders?: boolean;
}

/**
 * ConnectionState tracks the current state of the SSE connection.
 */
export type ConnectionState = "disconnected" | "connecting" | "connected" | "error";

/**
 * ConnectionStatus provides detailed information about the connection state.
 */
export interface ConnectionStatus {
	/** Current state of the connection */
	state: ConnectionState;

	/** Human-readable message about the connection status */
	message: string;

	/** Timestamp of the last state change */
	lastUpdate: Date;

	/** Number of events received in current session */
	eventsReceived: number;

	/** Number of events successfully processed */
	eventsProcessed: number;

	/** Number of errors encountered */
	errorCount: number;
}

/**
 * EventProcessingResult represents the outcome of processing a single event.
 */
export interface EventProcessingResult {
	/** Whether the event was successfully processed */
	success: boolean;

	/** The event that was processed */
	event: WebhookEvent;

	/** Error message if processing failed */
	error?: string;

	/** Path where the file was written/updated */
	filePath?: string;

	/** Whether the event was acknowledged to the server */
	acknowledged: boolean;
}

/**
 * PollingOptions configures the polling handler behavior.
 */
export interface PollingOptions {
	/** Interval between poll requests in milliseconds */
	intervalMs: number;

	/** Whether polling is currently enabled */
	enabled: boolean;

	/** Maximum number of events to fetch per poll */
	limit: number;
}

/**
 * FileOperationOptions configures how files are written/updated.
 */
export interface FileOperationOptions {
	/** Write mode for the file */
	mode: "append" | "overwrite";

	/** Whether to create parent directories if they don't exist */
	createDirs: boolean;

	/** Optional custom content separator for append mode */
	separator?: string;
}

