/**
 * settings_tab.ts - Plugin Settings UI (v3.0 - Simplified)
 *
 * Implements Progressive Disclosure principle:
 * - Only 2 essential fields visible by default
 * - Advanced settings collapsed
 * - Smart defaults that "just work"
 */

import { App, Notice, PluginSettingTab, Setting, Platform } from "obsidian";
import type ObsidianWebhooksPlugin from "./main";

export class WebhookSettingTab extends PluginSettingTab {
	plugin: ObsidianWebhooksPlugin;
	private advancedExpanded = false;

	constructor(app: App, plugin: ObsidianWebhooksPlugin) {
		super(app, plugin);
		this.plugin = plugin;
	}

	display(): void {
		const { containerEl } = this;
		containerEl.empty();

		containerEl.createEl("h2", { text: "Obsidian Webhooks" });

		// Status indicator at the top (most important)
		this.renderStatusIndicator(containerEl);

		// Client Key (only essential setting)
		this.renderClientKey(containerEl);

		// Test Connection button
		this.renderTestConnection(containerEl);

		// Advanced Settings (collapsed by default)
		this.renderAdvancedSettings(containerEl);
	}

	private renderStatusIndicator(containerEl: HTMLElement): void {
		const status = this.plugin.connectionStatus;

		// Determine emoji and style based on state
		let emoji = "🔴";
		let className = "webhook-status-error";
		let message = status.message;

		if (status.state === "connected") {
			emoji = "🟢";
			className = "webhook-status-connected";
			message = `Connected (${status.eventsReceived} events received)`;
		} else if (status.state === "connecting") {
			emoji = "🟡";
			className = "webhook-status-warning";
		} else if (status.message.includes("Reconnecting")) {
			emoji = "🟠";
			className = "webhook-status-warning";
		}

		const statusDiv = containerEl.createDiv({
			cls: `webhook-status-banner ${className}`,
		});
		statusDiv.createEl("span", {
			text: `${emoji} ${message}`,
			cls: "webhook-status-text",
		});

		// Add some CSS for the status banner
		const style = containerEl.createEl("style");
		style.textContent = `
			.webhook-status-banner {
				padding: 12px 16px;
				border-radius: 6px;
				margin: 16px 0;
				font-weight: 500;
			}
			.webhook-status-connected {
				background: #d4edda;
				color: #155724;
				border: 1px solid #c3e6cb;
			}
			.webhook-status-error {
				background: #f8d7da;
				color: #721c24;
				border: 1px solid #f5c6cb;
			}
			.webhook-status-warning {
				background: #fff3cd;
				color: #856404;
				border: 1px solid #ffeaa7;
			}
			.webhook-test-result {
				margin-top: 8px;
				padding: 8px 12px;
				border-radius: 4px;
				font-size: 0.9em;
			}
			.test-success {
				background: #d4edda;
				color: #155724;
			}
			.test-error {
				background: #f8d7da;
				color: #721c24;
			}
		`;
	}

	private renderClientKey(containerEl: HTMLElement): void {
		new Setting(containerEl)
			.setName("Client Key")
			.setDesc("Get your key from the dashboard")
			.addText((text) =>
				text
					.setPlaceholder("ck_...")
					.setValue(this.plugin.settings.clientKey)
					.onChange(async (value) => {
						this.plugin.settings.clientKey = value.trim();
						await this.plugin.saveSettings();
					})
			);
	}

	private renderTestConnection(containerEl: HTMLElement): void {
		const setting = new Setting(containerEl)
			.setName("Test Connection")
			.setDesc("Verify your client key and server connection")
			.addButton((button) => {
				button
					.setButtonText("Test")
					.onClick(async () => {
						button.setDisabled(true);
						button.setButtonText("Testing...");
						await this.testConnection(setting.settingEl);
						button.setDisabled(false);
						button.setButtonText("Test");
					});
			});
	}

	private async testConnection(settingEl: HTMLElement): Promise<void> {
		// Remove previous result if exists
		const existingResult = settingEl.querySelector(".webhook-test-result");
		if (existingResult) {
			existingResult.remove();
		}

		const clientKey = this.plugin.settings.clientKey;
		if (!clientKey) {
			const resultDiv = settingEl.createDiv({ cls: "webhook-test-result test-error" });
			resultDiv.setText("Enter your client key first");
			return;
		}

		const resultDiv = settingEl.createDiv({ cls: "webhook-test-result" });

		try {
			const startTime = Date.now();
			const response = await fetch(
				`${this.plugin.settings.serverUrl}/test/${clientKey}`,
				{
					method: "POST",
					signal: AbortSignal.timeout(5000),
				}
			);

			if (!response.ok) {
				const errorMsg = response.status === 401 || response.status === 404
					? "Invalid client key"
					: `Server error: ${response.statusText}`;
				resultDiv.setText(`Test failed: ${errorMsg}`);
				resultDiv.addClass("test-error");
				return;
			}

			const latency = Date.now() - startTime;
			resultDiv.setText(`Test passed! Server responded in ${latency}ms`);
			resultDiv.addClass("test-success");

			// Auto-hide result after 10 seconds
			setTimeout(() => {
				resultDiv.remove();
			}, 10000);
		} catch (error) {
			const errorMsg = error instanceof Error
				? (error.name === "TimeoutError" ? "Connection timeout" : error.message)
				: "Network error";
			resultDiv.setText(`Test failed: ${errorMsg}`);
			resultDiv.addClass("test-error");
		}
	}

	private renderAdvancedSettings(containerEl: HTMLElement): void {
		// Collapsible Advanced Settings section
		const advancedSetting = new Setting(containerEl)
			.setName("Advanced Settings")
			.setHeading()
			.addExtraButton((button) => {
				button
					.setIcon(this.advancedExpanded ? "chevron-down" : "chevron-right")
					.setTooltip(this.advancedExpanded ? "Collapse" : "Expand")
					.onClick(() => {
						this.advancedExpanded = !this.advancedExpanded;
						this.display(); // Re-render
					});
			});

		if (!this.advancedExpanded) {
			return;
		}

		// Server URL (for self-hosted setups)
		new Setting(containerEl)
			.setName("Server URL")
			.setDesc("💡 Change only for self-hosted setup")
			.addText((text) =>
				text
					.setPlaceholder("https://obsidian-webhooks.khabaroff.studio")
					.setValue(this.plugin.settings.serverUrl)
					.onChange(async (value) => {
						// Normalize: remove trailing slash
						this.plugin.settings.serverUrl = value.trim().replace(/\/$/, "");
						await this.plugin.saveSettings();
					})
			);

		// Auto-reconnect
		new Setting(containerEl)
			.setName("Auto-reconnect")
			.setDesc("Automatically connect when plugin loads")
			.addToggle((toggle) =>
				toggle
					.setValue(this.plugin.settings.autoConnect)
					.onChange(async (value) => {
						this.plugin.settings.autoConnect = value;
						await this.plugin.saveSettings();
					})
			);

		// Write mode
		new Setting(containerEl)
			.setName("Write mode")
			.setDesc("How to write content to files")
			.addDropdown((dropdown) =>
				dropdown
					.addOption("append", "Append to end")
					.addOption("overwrite", "Overwrite file")
					.setValue(this.plugin.settings.defaultMode)
					.onChange(async (value) => {
						this.plugin.settings.defaultMode = value as "append" | "overwrite";
						await this.plugin.saveSettings();
					})
			);

		// Debug logging
		new Setting(containerEl)
			.setName("Debug logging")
			.setDesc("Enable console output for troubleshooting")
			.addToggle((toggle) =>
				toggle
					.setValue(this.plugin.settings.enableDebugLogging)
					.onChange(async (value) => {
						this.plugin.settings.enableDebugLogging = value;
						await this.plugin.saveSettings();
					})
			);

		// Quick Actions
		containerEl.createEl("h4", { text: "🔗 Quick Actions" });

		// Open Dashboard button
		new Setting(containerEl)
			.setName("User Dashboard")
			.setDesc("View your keys and webhook logs")
			.addButton((button) => {
				button
					.setButtonText("Open Dashboard")
					.onClick(() => {
						const clientKey = this.plugin.settings.clientKey;
						if (!clientKey) {
							new Notice("Please configure your client key first");
							return;
						}
						window.open(`${this.plugin.settings.serverUrl}/dashboard`, "_blank");
					});
			});

		// Statistics
		containerEl.createEl("h4", { text: "📊 Statistics" });
		const status = this.plugin.connectionStatus;
		const statsDiv = containerEl.createDiv({ cls: "webhook-statistics" });
		statsDiv.createEl("p", { text: `• Events received: ${status.eventsReceived}` });
		statsDiv.createEl("p", { text: `• Events processed: ${status.eventsProcessed}` });
		statsDiv.createEl("p", { text: `• Errors: ${status.errorCount}` });
	}

}
