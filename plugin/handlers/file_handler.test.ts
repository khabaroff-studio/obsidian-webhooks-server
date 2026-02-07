/**
 * file_handler.test.ts - Unit tests for FileHandler
 *
 * Following TDD approach (RED-GREEN-REFACTOR):
 * 1. Write failing tests first
 * 2. Implement minimal code to pass
 * 3. Refactor for quality
 */

import { describe, test, expect, beforeEach, mock } from "bun:test";
import { FileHandler } from "./file_handler";
import type { WebhookEvent, FileOperationOptions } from "../types";

// Mock Obsidian Vault API
interface MockFile {
	path: string;
	extension: string;
}

interface MockVault {
	getAbstractFileByPath: ReturnType<typeof mock>;
	create: ReturnType<typeof mock>;
	modify: ReturnType<typeof mock>;
	read: ReturnType<typeof mock>;
	createFolder: ReturnType<typeof mock>;
}

describe("FileHandler", () => {
	let mockVault: MockVault;
	let fileHandler: FileHandler;

	beforeEach(() => {
		// Create fresh mocks for each test
		mockVault = {
			getAbstractFileByPath: mock(() => null),
			create: mock(async () => {}),
			modify: mock(async () => {}),
			read: mock(async () => ""),
			createFolder: mock(async () => {}),
		};

		fileHandler = new FileHandler(mockVault as any);
	});

	describe("File Creation", () => {
		test("should create new file when it doesn't exist", async () => {
			const event: WebhookEvent = {
				id: "test-event-1",
				path: "inbox/note.md",
				data: "Hello, World!",
				created_at: new Date().toISOString(),
			};

			mockVault.getAbstractFileByPath.mockReturnValue(null);

			await fileHandler.processEvent(event);

			expect(mockVault.create).toHaveBeenCalledWith(
				"inbox/note.md",
				"Hello, World!"
			);
			expect(mockVault.create).toHaveBeenCalledTimes(1);
		});

		test("should create parent directories automatically", async () => {
			const event: WebhookEvent = {
				id: "test-event-2",
				path: "inbox/emails/important.md",
				data: "Important message",
				created_at: new Date().toISOString(),
			};

			mockVault.getAbstractFileByPath.mockReturnValue(null);

			await fileHandler.processEvent(event);

			expect(mockVault.createFolder).toHaveBeenCalledWith("inbox/emails");
			expect(mockVault.create).toHaveBeenCalledWith(
				"inbox/emails/important.md",
				"Important message"
			);
		});

		test("should handle file in root directory (no parent)", async () => {
			const event: WebhookEvent = {
				id: "test-event-3",
				path: "note.md",
				data: "Root note",
				created_at: new Date().toISOString(),
			};

			mockVault.getAbstractFileByPath.mockReturnValue(null);

			await fileHandler.processEvent(event);

			expect(mockVault.createFolder).not.toHaveBeenCalled();
			expect(mockVault.create).toHaveBeenCalledWith("note.md", "Root note");
		});
	});

	describe("Append Mode", () => {
		test("should append to existing file", async () => {
			const event: WebhookEvent = {
				id: "test-event-4",
				path: "inbox/note.md",
				data: "New content",
				created_at: new Date().toISOString(),
			};

			const mockFile: MockFile = {
				path: "inbox/note.md",
				extension: "md",
			};

			mockVault.getAbstractFileByPath.mockReturnValue(mockFile);
			mockVault.read.mockResolvedValue("Existing content");

			const options: FileOperationOptions = {
				mode: "append",
				createDirs: true,
			};

			await fileHandler.processEvent(event, options);

			expect(mockVault.read).toHaveBeenCalledWith(mockFile);
			expect(mockVault.modify).toHaveBeenCalledWith(
				mockFile,
				"Existing contentNew content"
			);
		});

		test("should use custom separator in append mode", async () => {
			const event: WebhookEvent = {
				id: "test-event-5",
				path: "inbox/note.md",
				data: "New content",
				created_at: new Date().toISOString(),
			};

			const mockFile: MockFile = {
				path: "inbox/note.md",
				extension: "md",
			};

			mockVault.getAbstractFileByPath.mockReturnValue(mockFile);
			mockVault.read.mockResolvedValue("Existing content");

			const options: FileOperationOptions = {
				mode: "append",
				createDirs: true,
				separator: "\n---\n",
			};

			await fileHandler.processEvent(event, options);

			expect(mockVault.modify).toHaveBeenCalledWith(
				mockFile,
				"Existing content\n---\nNew content"
			);
		});
	});

	describe("Overwrite Mode", () => {
		test("should overwrite existing file", async () => {
			const event: WebhookEvent = {
				id: "test-event-6",
				path: "inbox/note.md",
				data: "New content",
				created_at: new Date().toISOString(),
			};

			const mockFile: MockFile = {
				path: "inbox/note.md",
				extension: "md",
			};

			mockVault.getAbstractFileByPath.mockReturnValue(mockFile);

			const options: FileOperationOptions = {
				mode: "overwrite",
				createDirs: true,
			};

			await fileHandler.processEvent(event, options);

			expect(mockVault.read).not.toHaveBeenCalled();
			expect(mockVault.modify).toHaveBeenCalledWith(mockFile, "New content");
		});

		test("should create file in overwrite mode if it doesn't exist", async () => {
			const event: WebhookEvent = {
				id: "test-event-7",
				path: "inbox/note.md",
				data: "New content",
				created_at: new Date().toISOString(),
			};

			mockVault.getAbstractFileByPath.mockReturnValue(null);

			const options: FileOperationOptions = {
				mode: "overwrite",
				createDirs: true,
			};

			await fileHandler.processEvent(event, options);

			expect(mockVault.create).toHaveBeenCalledWith(
				"inbox/note.md",
				"New content"
			);
		});
	});

	describe("Error Handling", () => {
		test("should throw error when event has no data", async () => {
			const event: WebhookEvent = {
				id: "test-event-8",
				path: "inbox/note.md",
				data: undefined,
				created_at: new Date().toISOString(),
			};

			await expect(fileHandler.processEvent(event)).rejects.toThrow(
				"Event has no data to write"
			);
		});

		test("should throw error when path is empty", async () => {
			const event: WebhookEvent = {
				id: "test-event-9",
				path: "",
				data: "Content",
				created_at: new Date().toISOString(),
			};

			await expect(fileHandler.processEvent(event)).rejects.toThrow(
				"Event has no path specified"
			);
		});

		test("should propagate vault errors", async () => {
			const event: WebhookEvent = {
				id: "test-event-10",
				path: "inbox/note.md",
				data: "Content",
				created_at: new Date().toISOString(),
			};

			mockVault.getAbstractFileByPath.mockReturnValue(null);
			mockVault.create.mockRejectedValue(new Error("Permission denied"));

			await expect(fileHandler.processEvent(event)).rejects.toThrow(
				"Permission denied"
			);
		});

		test("should handle folder creation errors gracefully", async () => {
			const event: WebhookEvent = {
				id: "test-event-11",
				path: "inbox/note.md",
				data: "Content",
				created_at: new Date().toISOString(),
			};

			mockVault.getAbstractFileByPath.mockReturnValue(null);
			mockVault.createFolder.mockRejectedValue(
				new Error("Folder already exists")
			);

			// Should not throw - folder creation errors are non-fatal
			await fileHandler.processEvent(event);
			expect(mockVault.create).toHaveBeenCalled();
		});
	});

	describe("Newline Handling", () => {
		test("should not add newlines by default", async () => {
			const event: WebhookEvent = {
				id: "test-event-12",
				path: "note.md",
				data: "Content",
				created_at: new Date().toISOString(),
			};

			mockVault.getAbstractFileByPath.mockReturnValue(null);

			await fileHandler.processEvent(event);

			expect(mockVault.create).toHaveBeenCalledWith("note.md", "Content");
		});

		test("should add unix newline when specified", async () => {
			const event: WebhookEvent = {
				id: "test-event-13",
				path: "note.md",
				data: "Content",
				created_at: new Date().toISOString(),
			};

			mockVault.getAbstractFileByPath.mockReturnValue(null);

			await fileHandler.processEvent(event, {
				mode: "append",
				createDirs: true,
				separator: "\n",
			});

			// When creating new file with separator, separator is added as suffix
			expect(mockVault.create).toHaveBeenCalledWith("note.md", "Content\n");
		});

		test("should add windows newline when specified", async () => {
			const event: WebhookEvent = {
				id: "test-event-14",
				path: "note.md",
				data: "Content",
				created_at: new Date().toISOString(),
			};

			mockVault.getAbstractFileByPath.mockReturnValue(null);

			await fileHandler.processEvent(event, {
				mode: "append",
				createDirs: true,
				separator: "\r\n",
			});

			expect(mockVault.create).toHaveBeenCalledWith("note.md", "Content\r\n");
		});
	});
});
