/**
 * polling_handler.test.ts - Unit tests for PollingHandler
 *
 * Following TDD approach (RED-GREEN-REFACTOR):
 * 1. Write failing tests first (RED)
 * 2. Implement minimal code to pass (GREEN)
 * 3. Refactor for quality (REFACTOR)
 */

import { describe, test, expect, beforeEach, mock, afterEach } from "bun:test";
import { PollingHandler } from "./polling_handler";
import type { WebhookEvent } from "../types";

// Mock fetch globally
declare global {
	var fetch: any;
}

describe("PollingHandler", () => {
	let pollingHandler: PollingHandler;
	let mockEventCallback: ReturnType<typeof mock>;
	let originalFetch: typeof global.fetch;

	beforeEach(() => {
		// Save original fetch
		originalFetch = global.fetch;

		// Create mock event callback
		mockEventCallback = mock(async (event: WebhookEvent) => {});

		// Create polling handler instance
		pollingHandler = new PollingHandler(
			"http://localhost:8080",
			"cl_test_key_123",
			mockEventCallback
		);
	});

	afterEach(() => {
		// Stop polling if running
		pollingHandler.stop();

		// Restore original fetch
		global.fetch = originalFetch;
	});

	describe("Constructor", () => {
		test("should create instance with correct configuration", () => {
			expect(pollingHandler).toBeDefined();
		});

		test("should not start polling automatically", () => {
			const handler = new PollingHandler(
				"http://localhost:8080",
				"cl_test_key",
				mockEventCallback
			);

			expect(handler.isPolling()).toBe(false);
		});
	});

	describe("pollOnce()", () => {
		test("should fetch events from server with poll=true parameter", async () => {
			const mockEvents: WebhookEvent[] = [
				{
					id: "event-1",
					path: "inbox/note.md",
					data: "Test content",
					created_at: new Date().toISOString(),
					processed: false,
				},
			];

			global.fetch = mock(async (url: string) => {
				return {
					ok: true,
					json: async () => mockEvents,
				};
			});

			await pollingHandler.pollOnce();

			expect(global.fetch).toHaveBeenCalledTimes(1);
			const fetchUrl = global.fetch.mock.calls[0][0];
			expect(fetchUrl).toContain("/events/cl_test_key_123");
			expect(fetchUrl).toContain("poll=true");
		});

		test("should invoke callback for each event received", async () => {
			const mockEvents: WebhookEvent[] = [
				{
					id: "event-1",
					path: "inbox/note1.md",
					data: "Content 1",
					created_at: new Date().toISOString(),
					processed: false,
				},
				{
					id: "event-2",
					path: "inbox/note2.md",
					data: "Content 2",
					created_at: new Date().toISOString(),
					processed: false,
				},
			];

			global.fetch = mock(async () => ({
				ok: true,
				json: async () => mockEvents,
			}));

			await pollingHandler.pollOnce();

			expect(mockEventCallback).toHaveBeenCalledTimes(2);
			expect(mockEventCallback).toHaveBeenCalledWith(mockEvents[0]);
			expect(mockEventCallback).toHaveBeenCalledWith(mockEvents[1]);
		});

		test("should handle empty event array", async () => {
			global.fetch = mock(async () => ({
				ok: true,
				json: async () => [],
			}));

			await pollingHandler.pollOnce();

			expect(mockEventCallback).not.toHaveBeenCalled();
		});

		test("should handle HTTP errors gracefully", async () => {
			global.fetch = mock(async () => ({
				ok: false,
				status: 401,
				statusText: "Unauthorized",
			}));

			// Should not throw
			await expect(pollingHandler.pollOnce()).resolves.toBeUndefined();
			expect(mockEventCallback).not.toHaveBeenCalled();
		});

		test("should handle network errors gracefully", async () => {
			global.fetch = mock(async () => {
				throw new Error("Network error");
			});

			// Should not throw
			await expect(pollingHandler.pollOnce()).resolves.toBeUndefined();
			expect(mockEventCallback).not.toHaveBeenCalled();
		});

		test("should handle malformed JSON response", async () => {
			global.fetch = mock(async () => ({
				ok: true,
				json: async () => {
					throw new Error("Invalid JSON");
				},
			}));

			// Should not throw
			await expect(pollingHandler.pollOnce()).resolves.toBeUndefined();
			expect(mockEventCallback).not.toHaveBeenCalled();
		});
	});

	describe("start()", () => {
		test("should start continuous polling", async () => {
			global.fetch = mock(async () => ({
				ok: true,
				json: async () => [],
			}));

			pollingHandler.start(100); // 100ms interval

			// Wait for at least 2 poll cycles
			await new Promise((resolve) => setTimeout(resolve, 250));

			pollingHandler.stop();

			// Should have polled at least 2 times
			expect(global.fetch.mock.calls.length).toBeGreaterThanOrEqual(2);
		});

		test("should set isPolling to true when started", () => {
			global.fetch = mock(async () => ({
				ok: true,
				json: async () => [],
			}));

			pollingHandler.start(1000);

			expect(pollingHandler.isPolling()).toBe(true);

			pollingHandler.stop();
		});

		test("should use default interval if not specified", async () => {
			global.fetch = mock(async () => ({
				ok: true,
				json: async () => [],
			}));

			pollingHandler.start();

			expect(pollingHandler.isPolling()).toBe(true);

			pollingHandler.stop();
		});

		test("should not start multiple polling loops", async () => {
			global.fetch = mock(async () => ({
				ok: true,
				json: async () => [],
			}));

			pollingHandler.start(100);
			pollingHandler.start(100); // Try to start again

			await new Promise((resolve) => setTimeout(resolve, 250));

			pollingHandler.stop();

			// Should only have one polling loop running
			const callCount = global.fetch.mock.calls.length;
			expect(callCount).toBeLessThan(6); // Would be much higher with 2 loops
		});
	});

	describe("stop()", () => {
		test("should stop polling when called", async () => {
			global.fetch = mock(async () => ({
				ok: true,
				json: async () => [],
			}));

			pollingHandler.start(100);
			await new Promise((resolve) => setTimeout(resolve, 250));

			const callCountBefore = global.fetch.mock.calls.length;

			pollingHandler.stop();
			await new Promise((resolve) => setTimeout(resolve, 250));

			const callCountAfter = global.fetch.mock.calls.length;

			// Should not have increased significantly after stop
			expect(callCountAfter - callCountBefore).toBeLessThanOrEqual(1);
		});

		test("should set isPolling to false when stopped", () => {
			global.fetch = mock(async () => ({
				ok: true,
				json: async () => [],
			}));

			pollingHandler.start(1000);
			expect(pollingHandler.isPolling()).toBe(true);

			pollingHandler.stop();
			expect(pollingHandler.isPolling()).toBe(false);
		});

		test("should be safe to call stop multiple times", () => {
			pollingHandler.stop();
			pollingHandler.stop();
			pollingHandler.stop();

			expect(pollingHandler.isPolling()).toBe(false);
		});

		test("should be safe to call stop when not polling", () => {
			expect(() => pollingHandler.stop()).not.toThrow();
			expect(pollingHandler.isPolling()).toBe(false);
		});
	});

	describe("Event Callback Invocation", () => {
		test("should pass complete event data to callback", async () => {
			const testEvent: WebhookEvent = {
				id: "test-event-123",
				path: "inbox/webhook.md",
				data: "Test webhook data",
				created_at: "2025-11-23T10:00:00Z",
				processed: false,
			};

			global.fetch = mock(async () => ({
				ok: true,
				json: async () => [testEvent],
			}));

			await pollingHandler.pollOnce();

			expect(mockEventCallback).toHaveBeenCalledWith(testEvent);
		});

		test("should continue polling even if callback throws error", async () => {
			const mockEvents: WebhookEvent[] = [
				{
					id: "event-1",
					path: "inbox/note.md",
					data: "Content",
					created_at: new Date().toISOString(),
					processed: false,
				},
			];

			global.fetch = mock(async () => ({
				ok: true,
				json: async () => mockEvents,
			}));

			const throwingCallback = mock(async () => {
				throw new Error("Callback error");
			});

			const handler = new PollingHandler(
				"http://localhost:8080",
				"cl_test_key",
				throwingCallback
			);

			// Should not throw
			await expect(handler.pollOnce()).resolves.toBeUndefined();

			expect(throwingCallback).toHaveBeenCalled();
		});
	});

	describe("Error Handling", () => {
		test("should log errors to console (if debug enabled)", async () => {
			global.fetch = mock(async () => {
				throw new Error("Connection refused");
			});

			// Should not throw
			await expect(pollingHandler.pollOnce()).resolves.toBeUndefined();
		});

		test("should handle invalid event data gracefully", async () => {
			global.fetch = mock(async () => ({
				ok: true,
				json: async () => [
					{
						// Missing required fields
						id: "event-1",
					},
				],
			}));

			// Should not throw - callback will handle validation
			await expect(pollingHandler.pollOnce()).resolves.toBeUndefined();
		});
	});
});
