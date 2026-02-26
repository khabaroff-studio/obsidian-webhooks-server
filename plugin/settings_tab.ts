/**
 * settings_tab.ts - Plugin Settings UI (v3.0 - Simplified)
 *
 * Implements Progressive Disclosure principle:
 * - Only 2 essential fields visible by default
 * - Advanced settings collapsed
 * - Smart defaults that "just work"
 */

import { App, Notice, PluginSettingTab, Setting } from "obsidian";
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

		let className = "webhook-status-error";
		let message = status.message;

		if (status.state === "connected") {
			className = "webhook-status-connected";
			message = "Connected";
		} else if (status.state === "connecting") {
			className = "webhook-status-warning";
		} else if (status.message.includes("Reconnecting")) {
			className = "webhook-status-warning";
		}

		const statusDiv = containerEl.createDiv({
			cls: `webhook-status-banner ${className}`,
		});
		statusDiv.createEl("span", {
			text: message,
			cls: "webhook-status-text",
		});
	}

	private renderClientKey(containerEl: HTMLElement): void {
		new Setting(containerEl)
			.setName("Client key")
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
			.setName("Test connection")
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
		// Remove previous result if exists (search in parent since result is a sibling)
		const parent = settingEl.parentElement;
		const existingResult = parent?.querySelector(".webhook-test-result");
		if (existingResult) {
			existingResult.remove();
		}

		const clientKey = this.plugin.settings.clientKey;
		if (!clientKey) {
			const resultDiv = this.createResultDiv(settingEl, "test-error");
			resultDiv.textContent = "Enter your client key first";
			return;
		}

		const resultDiv = this.createResultDiv(settingEl);

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
				resultDiv.textContent = `Test failed: ${errorMsg}`;
				resultDiv.classList.add("test-error");
				return;
			}

			const latency = Date.now() - startTime;
			resultDiv.textContent = `Test passed! Server responded in ${latency}ms`;
			resultDiv.classList.add("test-success");

			// Auto-hide result after 10 seconds
			setTimeout(() => {
				resultDiv.remove();
			}, 10000);
		} catch (error) {
			const errorMsg = error instanceof Error
				? (error.name === "TimeoutError" ? "Connection timeout" : error.message)
				: "Network error";
			resultDiv.textContent = `Test failed: ${errorMsg}`;
			resultDiv.classList.add("test-error");
		}
	}

	private createResultDiv(settingEl: HTMLElement, ...classes: string[]): HTMLElement {
		const div = document.createElement("div");
		div.className = ["webhook-test-result", ...classes].join(" ");
		settingEl.insertAdjacentElement("afterend", div);
		return div;
	}

	private renderAdvancedSettings(containerEl: HTMLElement): void {
		// Collapsible Advanced Settings section
		const advancedSetting = new Setting(containerEl)
			.setName("Advanced settings")
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
			.setDesc("Change only for self-hosted setup")
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

		// Quick actions
		new Setting(containerEl)
			.setName("Quick actions")
			.setHeading();

		// Open Dashboard button
		new Setting(containerEl)
			.setName("User dashboard")
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

	}

}
