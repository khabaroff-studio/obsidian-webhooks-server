/**
 * file_handler.ts - File operations handler for webhook events
 *
 * Handles creating/updating files in the Obsidian vault based on webhook events.
 * Supports append/overwrite modes, directory auto-creation, and custom separators.
 */

import type { WebhookEvent, FileOperationOptions } from "../types";
import type { Vault, TFile } from "obsidian";
import { normalizePath } from "obsidian";
import { formatData } from "../utils/json-formatter";

/**
 * Default options for file operations
 */
const DEFAULT_OPTIONS: FileOperationOptions = {
	mode: "append",
	createDirs: true,
};

/**
 * FileHandler processes webhook events and writes them to vault files
 */
export class FileHandler {
	private vault: Vault;

	constructor(vault: Vault) {
		this.vault = vault;
	}

	/**
	 * Process a webhook event and write it to the vault
	 *
	 * @param event - The webhook event to process
	 * @param options - Optional file operation settings
	 * @throws Error if event is invalid or file operation fails
	 */
	async processEvent(
		event: WebhookEvent,
		options?: FileOperationOptions
	): Promise<void> {
		// Validate event
		this.validateEvent(event);

		// Normalize the file path (required by Obsidian guidelines)
		const path = normalizePath(event.path);

		// Merge options with defaults
		const opts = { ...DEFAULT_OPTIONS, ...options };

		// Prepare content with optional separator
		const content = this.prepareContent(event.data!, opts);

		// Ensure parent directory exists
		await this.ensureDirectoryExists(path);

		// Use adapter.exists() for reliable file existence check (doesn't rely on cache)
		const fileExists = await this.vault.adapter.exists(path);

		if (fileExists) {
			// File exists - get it and update
			const existingFile = this.vault.getAbstractFileByPath(path);
			if (existingFile && this.isFile(existingFile)) {
				await this.updateExistingFile(existingFile as TFile, content, opts);
			} else {
				// Fallback: file exists but not in cache - force update via adapter
				const existingContent = await this.vault.adapter.read(path);
				const newContent = opts.mode === "overwrite"
					? content
					: this.mergeContent(existingContent, content, opts);
				await this.vault.adapter.write(path, newContent);
			}
		} else {
			// File doesn't exist - create it
			await this.createNewFile(path, content);
		}
	}

	/**
	 * Validate that the event has required fields
	 */
	private validateEvent(event: WebhookEvent): void {
		if (!event.path || event.path.trim() === "") {
			throw new Error("Event has no path specified");
		}

		if (event.data === undefined || event.data === null) {
			throw new Error("Event has no data to write");
		}
	}

	/**
	 * Prepare content with optional separator and JSON formatting
	 */
	private prepareContent(
		data: string,
		options: FileOperationOptions
	): string {
		// Format JSON data to Markdown if applicable
		let content = formatData(data, {
			enabled: true,
			prettyPrintUnknown: false,
		});

		// Add separator as suffix when creating new files or in append mode
		if (options.separator) {
			return content + options.separator;
		}
		return content;
	}

	/**
	 * Ensure parent directory exists for the given path
	 */
	private async ensureDirectoryExists(filePath: string): Promise<void> {
		const dirPath = this.getDirectoryPath(filePath);

		// No directory to create (file is in root)
		if (!dirPath || dirPath === filePath) {
			return;
		}

		try {
			await this.vault.createFolder(dirPath);
		} catch (error) {
			// Folder might already exist - this is non-fatal
			// Only throw if it's a different error
			if (
				error instanceof Error &&
				!error.message.includes("already exists") &&
				!error.message.includes("Folder already exists")
			) {
				// Log but don't throw - we'll let file creation fail if there's a real issue
			}
		}
	}

	/**
	 * Extract directory path from file path
	 */
	private getDirectoryPath(filePath: string): string {
		const lastSlash = filePath.lastIndexOf("/");
		if (lastSlash === -1) {
			return ""; // File is in root
		}
		return filePath.substring(0, lastSlash);
	}

	/**
	 * Check if abstract file is a file (has extension property)
	 */
	private isFile(file: any): boolean {
		return file && file.hasOwnProperty("extension");
	}

	/**
	 * Update an existing file with new content
	 */
	private async updateExistingFile(
		file: TFile,
		content: string,
		options: FileOperationOptions
	): Promise<void> {
		if (options.mode === "overwrite") {
			// Overwrite mode - replace entire file
			await this.vault.modify(file, content);
		} else {
			// Append mode - add to end of file
			const existingContent = await this.vault.read(file);
			const newContent = this.mergeContent(existingContent, content, options);
			await this.vault.modify(file, newContent);
		}
	}

	/**
	 * Merge existing content with new content, handling separators
	 */
	private mergeContent(
		existingContent: string,
		newContent: string,
		options: FileOperationOptions
	): string {
		if (!options.separator) {
			return existingContent + newContent;
		}

		// Remove trailing separator from new content to avoid duplication
		const cleanedNewContent = newContent.replace(
			new RegExp(this.escapeRegex(options.separator) + "$"),
			""
		);

		// Don't prepend separator if existing content is empty
		// (preserves frontmatter at position 0)
		if (!existingContent) {
			return cleanedNewContent;
		}

		return existingContent + options.separator + cleanedNewContent;
	}

	/**
	 * Escape special regex characters
	 */
	private escapeRegex(str: string): string {
		return str.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
	}

	/**
	 * Create a new file with content
	 * If file already exists (race condition), fall back to updating it
	 */
	private async createNewFile(path: string, content: string): Promise<void> {
		try {
			await this.vault.create(path, content);
		} catch (error) {
			// Handle race condition: file was created between check and create
			if (error instanceof Error && error.message.includes("already exists")) {
				const existingFile = this.vault.getAbstractFileByPath(path);
				if (existingFile && this.isFile(existingFile)) {
					// File exists now - update it instead (append mode)
					const existingContent = await this.vault.read(existingFile as any);
					await this.vault.modify(existingFile as any, existingContent + content);
				} else {
					// File still doesn't exist - re-throw original error
					throw error;
				}
			} else {
				throw error;
			}
		}
	}
}
