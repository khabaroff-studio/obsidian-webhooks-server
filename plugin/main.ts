/**
 * main.ts - Obsidian Webhooks Selfhosted Plugin Entry Point
 *
 * This plugin receives webhook events from a self-hosted Go server via:
 * 1. Server-Sent Events (SSE) for real-time delivery
 * 2. Polling as a fallback for offline synchronization
 *
 * Features:
 * - Exactly-once delivery via ACK system
 * - Event deduplication using Set
 * - File creation/update in append or overwrite mode
 * - Automatic directory creation
 * - Connection status monitoring
 */

import { Notice, Plugin, Platform } from "obsidian";
import {
	WebhookEvent,
	WebhookSettings,
	ConnectionState,
	ConnectionStatus,
	EventProcessingResult,
} from "./types";
import { FileHandler } from "./handlers/file_handler";
import { ACKHandler } from "./handlers/ack_handler";
import { PollingHandler } from "./handlers/polling_handler";
import { SSEHandler } from "./handlers/sse_handler";
import { WebhookSettingTab } from "./settings_tab";

/**
 * Default settings for the plugin
 */
const DEFAULT_SETTINGS: WebhookSettings = {
	serverUrl: "https://obsidian-webhooks.khabaroff.studio",
	clientKey: "",
	autoConnect: true,
	defaultMode: "append",
	newlineType: "none",
	pollingInterval: 5,
	enablePolling: true,
	showNotifications: true,
	enableDebugLogging: false,
	maxRetries: 3,
	retryDelayMs: 1000,
	autoCreateFolders: true,
};

/**
 * Main plugin class that manages the webhook connection and event processing
 */
export default class ObsidianWebhooksPlugin extends Plugin {
	settings: WebhookSettings;
	processedEvents: Set<string> = new Set();
	connectionStatus: ConnectionStatus;
	statusBarItem: HTMLElement | null = null;

	// Handlers
	private fileHandler: FileHandler | null = null;
	private ackHandler: ACKHandler | null = null;
	private pollingHandler: PollingHandler | null = null;
	private sseHandler: SSEHandler | null = null;

	/**
	 * Called when the plugin is loaded
	 */
	async onload() {
		// Initialize connection status
		this.connectionStatus = {
			state: "disconnected",
			message: "Not connected",
			lastUpdate: new Date(),
			eventsReceived: 0,
			eventsProcessed: 0,
			errorCount: 0,
		};

		// Load settings from disk
		await this.loadSettings();

		// Initialize handlers
		this.initializeHandlers();

		// Add settings tab
		this.addSettingTab(new WebhookSettingTab(this.app, this));

		// Add status bar item
		this.statusBarItem = this.addStatusBarItem();
		this.updateStatusBar();

		// Auto-connect if enabled
		if (this.settings.autoConnect && this.settings.clientKey) {
			await this.connect();
		}

		this.log("Plugin loaded successfully");
	}

	/**
	 * Called when the plugin is unloaded
	 */
	onunload() {
		this.disconnect();
	}

	/**
	 * Load settings from disk
	 */
	async loadSettings() {
		const loadedData = (await this.loadData()) as Record<string, unknown> | null;

		// Migration: Handle v1 settings (endpoint → serverUrl)
		if (loadedData && "endpoint" in loadedData && !("serverUrl" in loadedData)) {
			loadedData.serverUrl = loadedData.endpoint;
			delete loadedData.endpoint;
			this.log("Migrated v1 settings: endpoint → serverUrl");
		}

		this.settings = Object.assign({}, DEFAULT_SETTINGS, loadedData) as WebhookSettings;

		// Apply smart defaults (v3.0 - override saved values)
		this.applySmartDefaults();
	}

	/**
	 * Apply smart defaults that work automatically (v3.0)
	 */
	private applySmartDefaults() {
		// Auto-detect newline type based on OS
		this.settings.newlineType = Platform.isWin ? "windows" : "unix";

		// Force enable polling (always on as SSE fallback)
		this.settings.enablePolling = true;
		this.settings.pollingInterval = 5;

		// Force enable notifications (essential user feedback)
		this.settings.showNotifications = true;

		// Force enable auto-create folders (prevent errors)
		this.settings.autoCreateFolders = true;

		// defaultMode is configurable in Advanced Settings (not forced)
	}

	/**
	 * Save settings to disk
	 */
	async saveSettings() {
		await this.saveData(this.settings);
	}

	/**
	 * Initialize all handlers with current settings
	 */
	initializeHandlers() {
		// File handler - handles writing to vault
		this.fileHandler = new FileHandler(this.app.vault);

		// ACK handler - handles acknowledgments to server
		this.ackHandler = new ACKHandler(
			this.settings.serverUrl,
			this.settings.clientKey,
			{
				onSuccess: (eventId: string) => {
					this.log(`ACK sent successfully for event ${eventId}`);
				},
				onError: (eventId: string, error: Error) => {
					this.log(`Failed to send ACK for event ${eventId}: ${error.message}`);
					this.connectionStatus.errorCount++;
					this.updateStatusBar();
				},
				onRetry: (eventId: string, attempt: number, error: Error) => {
					this.log(`Retrying ACK for event ${eventId} (attempt ${attempt}): ${error.message}`);
				},
			}
		);

		// Polling handler - handles polling for events
		this.pollingHandler = new PollingHandler(
			this.settings.serverUrl,
			this.settings.clientKey,
			(event: WebhookEvent) => this.handleEvent(event)
		);

		// SSE handler - handles real-time events
		this.sseHandler = new SSEHandler(
			this.settings.serverUrl,
			this.settings.clientKey,
			(event: WebhookEvent) => this.handleEvent(event),
			(state: string, message: string) => this.updateConnectionState(state as ConnectionState, message)
		);

		this.log("Handlers initialized");
	}

	/**
	 * Connect to the webhook server (SSE + optional polling)
	 */
	async connect() {
		if (!this.settings.clientKey) {
			new Notice("Please configure your client key in settings");
			return;
		}

		this.log("Connecting to webhook server...");
		this.updateConnectionState("connecting", "Establishing connection...");

		try {
			// First: Poll once to sync any offline events
			this.log("Performing initial poll for offline sync...");
			if (this.pollingHandler) {
				await this.pollingHandler.pollOnce();
			}

			// Then: Establish SSE connection for real-time events
			this.log("Establishing SSE connection...");
			if (this.sseHandler) {
				await this.sseHandler.connect();
			}

			// Start periodic polling if enabled
			if (this.settings.enablePolling && this.pollingHandler) {
				const intervalMs = this.settings.pollingInterval * 1000;
				this.log(`Starting polling with ${intervalMs}ms interval`);
				this.pollingHandler.start(intervalMs);
			}

			this.log("Connection established");

		} catch (error) {
			const errorMsg = error instanceof Error ? error.message : String(error);
			this.log(`Connection failed: ${errorMsg}`);
			this.updateConnectionState("error", `Connection failed: ${errorMsg}`);
			new Notice(`Failed to connect: ${errorMsg}`);
		}
	}

	/**
	 * Disconnect from the webhook server
	 */
	disconnect() {
		this.log("Disconnecting from webhook server...");

		// Close SSE connection
		if (this.sseHandler) {
			this.sseHandler.disconnect();
		}

		// Stop polling
		if (this.pollingHandler) {
			this.pollingHandler.stop();
		}

		this.updateConnectionState("disconnected", "Disconnected");
		this.log("Disconnected");
	}

	/**
	 * Handle incoming event from SSE or polling
	 * This is called by the event handlers
	 */
	private async handleEvent(event: WebhookEvent): Promise<void> {
		this.log(`Received event ${event.id} for path: ${event.path}`);
		this.connectionStatus.eventsReceived++;
		this.updateStatusBar();

		// Process the event
		const result = await this.processEvent(event);

		// Show notification if enabled
		if (this.settings.showNotifications && result.success) {
			new Notice(`Webhook event processed: ${event.path}`);
		}
	}

	/**
	 * Process a webhook event (write to file and send ACK)
	 */
	async processEvent(event: WebhookEvent): Promise<EventProcessingResult> {
		this.log(`Processing event ${event.id} for path: ${event.path}`);

		// Check for duplicates
		if (this.processedEvents.has(event.id)) {
			this.log(`Event ${event.id} already processed, skipping`);
			return {
				success: true,
				event,
				acknowledged: false,
			};
		}

		try {
			// Validate event data
			if (!event.data) {
				throw new Error("Event has no data to write");
			}

			// Calculate separator based on settings
			let separator: string | undefined;
			if (this.settings.newlineType === "windows") {
				separator = "\r\n";
			} else if (this.settings.newlineType === "unix") {
				separator = "\n";
			}

			// Write to file using FileHandler
			if (this.fileHandler) {
				await this.fileHandler.processEvent(event, {
					mode: this.settings.defaultMode,
					createDirs: true,
					separator: separator,
				});
			}

			this.log(`Successfully wrote to ${event.path}`);

			// Send ACK using ACKHandler
			let acknowledged = false;
			if (this.ackHandler) {
				acknowledged = await this.ackHandler.acknowledgeEvent(event.id);
			}

			// Mark as processed
			this.processedEvents.add(event.id);
			this.connectionStatus.eventsProcessed++;

			// Trim the processed events set if it gets too large
			if (this.processedEvents.size > 1000) {
				const entries = Array.from(this.processedEvents);
				this.processedEvents = new Set(entries.slice(-500));
			}

			this.updateStatusBar();

			return {
				success: true,
				event,
				filePath: event.path,
				acknowledged: acknowledged,
			};
		} catch (error) {
			this.connectionStatus.errorCount++;
			this.updateStatusBar();

			const errorMessage = error instanceof Error ? error.message : String(error);
			this.log(`Error processing event ${event.id}: ${errorMessage}`);

			// Show error notification
			if (this.settings.showNotifications) {
				new Notice(`Error processing webhook: ${errorMessage}`);
			}

			return {
				success: false,
				event,
				error: errorMessage,
				acknowledged: false,
			};
		}
	}

	/**
	 * Update connection state and notify UI
	 */
	updateConnectionState(state: ConnectionState, message: string) {
		this.connectionStatus.state = state;
		this.connectionStatus.message = message;
		this.connectionStatus.lastUpdate = new Date();
		this.updateStatusBar();
		this.log(`Connection state: ${state} - ${message}`);
	}

	/**
	 * Update the status bar item
	 */
	updateStatusBar() {
		if (!this.statusBarItem) return;

		const { state, eventsReceived, eventsProcessed, errorCount } = this.connectionStatus;
		let statusText = "";
		let statusClass = "";

		switch (state) {
			case "connected":
				statusText = `Webhooks: Connected (${eventsProcessed}/${eventsReceived})`;
				statusClass = "webhook-status-connected";
				break;
			case "connecting":
				statusText = "Webhooks: Connecting...";
				statusClass = "webhook-status-connecting";
				break;
			case "error":
				statusText = `Webhooks: Error (${errorCount})`;
				statusClass = "webhook-status-error";
				break;
			case "disconnected":
			default:
				statusText = "Webhooks: Disconnected";
				statusClass = "webhook-status-disconnected";
				break;
		}

		this.statusBarItem.setText(statusText);
		this.statusBarItem.className = `status-bar-item ${statusClass}`;
	}

	/**
	 * Log a message if debug logging is enabled
	 */
	log(message: string) {
		if (this.settings.enableDebugLogging) {
			console.debug(`[ObsidianWebhooks] ${message}`);
		}
	}
}

