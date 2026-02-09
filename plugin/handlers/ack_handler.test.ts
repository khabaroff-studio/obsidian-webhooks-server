/**
 * ack_handler.test.ts - Unit tests for ACKHandler
 *
 * Following TDD approach (RED-GREEN-REFACTOR):
 * 1. Write failing tests first
 * 2. Implement minimal code to pass
 * 3. Refactor for quality
 *
 * Tests cover:
 * - Successful ACK sending
 * - Error handling (no client key)
 * - Idempotency (duplicate ACK)
 * - Retry on network error
 */

import { describe, test, expect, beforeEach, mock, spyOn } from "bun:test";
import { ACKHandler } from "./ack_handler";

// Mock global fetch
const originalFetch = global.fetch;

describe("ACKHandler", () => {
	let ackHandler: ACKHandler;
	let mockFetch: ReturnType<typeof mock>;
	const testServerUrl = "https://webhooks.example.com";
	const testClientKey = "cl_test123456";

	beforeEach(() => {
		// Create fresh mock for fetch
		mockFetch = mock(async () => ({
			ok: true,
			status: 200,
			statusText: "OK",
		}));

		// Replace global fetch
		global.fetch = mockFetch as any;

		// Create fresh ACKHandler instance
		ackHandler = new ACKHandler(testServerUrl, testClientKey);
	});

	describe("Successful ACK Sending", () => {
		test("should send ACK to correct endpoint", async () => {
			const eventId = "event-123";

			await ackHandler.acknowledgeEvent(eventId);

			expect(mockFetch).toHaveBeenCalledTimes(1);
			expect(mockFetch).toHaveBeenCalledWith(
				`${testServerUrl}/ack/${testClientKey}/${eventId}`,
				{
					method: "POST",
					headers: {
						"Content-Type": "application/json",
					},
				}
			);
		});

		test("should return true on successful ACK", async () => {
			const eventId = "event-456";

			const result = await ackHandler.acknowledgeEvent(eventId);

			expect(result).toBe(true);
		});

		test("should handle successful ACK with different status codes (200, 204)", async () => {
			const eventId = "event-789";

			// Test 204 No Content
			mockFetch.mockResolvedValueOnce({
				ok: true,
				status: 204,
				statusText: "No Content",
			} as any);

			const result = await ackHandler.acknowledgeEvent(eventId);

			expect(result).toBe(true);
		});
	});

	describe("Error Handling - No Client Key", () => {
		test("should throw error when client key is empty", async () => {
			const handlerWithoutKey = new ACKHandler(testServerUrl, "");
			const eventId = "event-error-1";

			await expect(
				handlerWithoutKey.acknowledgeEvent(eventId)
			).rejects.toThrow("Cannot acknowledge event: no client key configured");

			expect(mockFetch).not.toHaveBeenCalled();
		});

		test("should throw error when client key is whitespace", async () => {
			const handlerWithoutKey = new ACKHandler(testServerUrl, "   ");
			const eventId = "event-error-2";

			await expect(
				handlerWithoutKey.acknowledgeEvent(eventId)
			).rejects.toThrow("Cannot acknowledge event: no client key configured");

			expect(mockFetch).not.toHaveBeenCalled();
		});
	});

	describe("Idempotency - Duplicate ACK", () => {
		test("should handle duplicate ACK gracefully (server returns 200)", async () => {
			const eventId = "event-duplicate-1";

			// First ACK
			const result1 = await ackHandler.acknowledgeEvent(eventId);
			expect(result1).toBe(true);

			// Second ACK (duplicate)
			const result2 = await ackHandler.acknowledgeEvent(eventId);
			expect(result2).toBe(true);

			expect(mockFetch).toHaveBeenCalledTimes(2);
		});

		test("should handle server 409 Conflict as success (already processed)", async () => {
			const eventId = "event-duplicate-2";

			mockFetch.mockResolvedValueOnce({
				ok: false,
				status: 409,
				statusText: "Conflict",
			} as any);

			// Should still return true because event is already processed
			const result = await ackHandler.acknowledgeEvent(eventId);
			expect(result).toBe(true);
		});
	});

	describe("Retry on Network Error", () => {
		test("should retry on network failure", async () => {
			const eventId = "event-retry-1";

			// First call fails with network error
			mockFetch.mockRejectedValueOnce(new Error("Network error"));

			// Second call succeeds
			mockFetch.mockResolvedValueOnce({
				ok: true,
				status: 200,
				statusText: "OK",
			} as any);

			const result = await ackHandler.acknowledgeEvent(eventId);

			expect(result).toBe(true);
			expect(mockFetch).toHaveBeenCalledTimes(2);
		});

		test("should retry on HTTP 5xx errors", async () => {
			const eventId = "event-retry-2";

			// First call fails with 500
			mockFetch.mockResolvedValueOnce({
				ok: false,
				status: 500,
				statusText: "Internal Server Error",
			} as any);

			// Second call succeeds
			mockFetch.mockResolvedValueOnce({
				ok: true,
				status: 200,
				statusText: "OK",
			} as any);

			const result = await ackHandler.acknowledgeEvent(eventId);

			expect(result).toBe(true);
			expect(mockFetch).toHaveBeenCalledTimes(2);
		});

		test("should fail after max retries exceeded", async () => {
			const eventId = "event-retry-3";

			// Mock setTimeout to avoid delays
			const originalSetTimeout = global.setTimeout;
			global.setTimeout = ((fn: Function) => {
				fn();
				return 0 as any;
			}) as any;

			// All calls fail
			mockFetch.mockRejectedValue(new Error("Network error"));

			const result = await ackHandler.acknowledgeEvent(eventId);

			// Restore setTimeout
			global.setTimeout = originalSetTimeout;

			expect(result).toBe(false);
			// Default max retries is 3, so 1 initial + 3 retries = 4 total calls
			expect(mockFetch).toHaveBeenCalledTimes(4);
		});

		test("should wait between retries with exponential backoff", async () => {
			const eventId = "event-retry-4";

			// Mock setTimeout to track delays
			const delays: number[] = [];
			const originalSetTimeout = global.setTimeout;
			global.setTimeout = ((fn: Function, delay: number) => {
				delays.push(delay);
				fn();
				return 0 as any;
			}) as any;

			// All calls fail
			mockFetch.mockRejectedValue(new Error("Network error"));

			await ackHandler.acknowledgeEvent(eventId);

			// Restore setTimeout
			global.setTimeout = originalSetTimeout;

			// Should have 3 delays (between 4 attempts)
			expect(delays.length).toBe(3);

			// Check exponential backoff: 1000ms, 2000ms, 4000ms
			expect(delays[0]).toBe(1000);
			expect(delays[1]).toBe(2000);
			expect(delays[2]).toBe(4000);
		});
	});

	describe("HTTP Error Handling", () => {
		test("should return false on HTTP 4xx client errors (except 409)", async () => {
			const eventId = "event-error-3";

			mockFetch.mockResolvedValueOnce({
				ok: false,
				status: 404,
				statusText: "Not Found",
			} as any);

			const result = await ackHandler.acknowledgeEvent(eventId);

			expect(result).toBe(false);
			// Should not retry on 4xx errors
			expect(mockFetch).toHaveBeenCalledTimes(1);
		});

		test("should return false on HTTP 401 Unauthorized", async () => {
			const eventId = "event-error-4";

			mockFetch.mockResolvedValueOnce({
				ok: false,
				status: 401,
				statusText: "Unauthorized",
			} as any);

			const result = await ackHandler.acknowledgeEvent(eventId);

			expect(result).toBe(false);
			expect(mockFetch).toHaveBeenCalledTimes(1);
		});
	});

	describe("Configuration", () => {
		test("should allow custom retry configuration", async () => {
			const customHandler = new ACKHandler(testServerUrl, testClientKey, {
				maxRetries: 2,
				retryDelayMs: 500,
			});

			const eventId = "event-config-1";

			// All calls fail
			mockFetch.mockRejectedValue(new Error("Network error"));

			await customHandler.acknowledgeEvent(eventId);

			// 1 initial + 2 retries = 3 total
			expect(mockFetch).toHaveBeenCalledTimes(3);
		});

		test("should allow disabling retries", async () => {
			const noRetryHandler = new ACKHandler(testServerUrl, testClientKey, {
				maxRetries: 0,
			});

			const eventId = "event-config-2";

			mockFetch.mockRejectedValue(new Error("Network error"));

			const result = await noRetryHandler.acknowledgeEvent(eventId);

			expect(result).toBe(false);
			// Only 1 attempt, no retries
			expect(mockFetch).toHaveBeenCalledTimes(1);
		});
	});

	describe("Logging Support", () => {
		test("should call onSuccess callback when ACK succeeds", async () => {
			const onSuccess = mock(() => {});
			const handlerWithCallbacks = new ACKHandler(
				testServerUrl,
				testClientKey,
				{
					onSuccess,
				}
			);

			const eventId = "event-log-1";

			await handlerWithCallbacks.acknowledgeEvent(eventId);

			expect(onSuccess).toHaveBeenCalledTimes(1);
			expect(onSuccess).toHaveBeenCalledWith(eventId);
		});

		test("should call onError callback when ACK fails", async () => {
			const onError = mock(() => {});
			const handlerWithCallbacks = new ACKHandler(
				testServerUrl,
				testClientKey,
				{
					maxRetries: 0,
					onError,
				}
			);

			const eventId = "event-log-2";

			mockFetch.mockRejectedValue(new Error("Network error"));

			await handlerWithCallbacks.acknowledgeEvent(eventId);

			expect(onError).toHaveBeenCalledTimes(1);
			expect(onError.mock.calls[0][0]).toBe(eventId);
			expect(onError.mock.calls[0][1]).toBeInstanceOf(Error);
		});

		test("should call onRetry callback on each retry attempt", async () => {
			const onRetry = mock(() => {});
			const handlerWithCallbacks = new ACKHandler(
				testServerUrl,
				testClientKey,
				{
					maxRetries: 2,
					onRetry,
				}
			);

			const eventId = "event-log-3";

			// Mock setTimeout to avoid delays
			const originalSetTimeout = global.setTimeout;
			global.setTimeout = ((fn: Function) => {
				fn();
				return 0 as any;
			}) as any;

			// Fail twice, then succeed
			mockFetch
				.mockRejectedValueOnce(new Error("Network error"))
				.mockRejectedValueOnce(new Error("Network error"))
				.mockResolvedValueOnce({
					ok: true,
					status: 200,
					statusText: "OK",
				} as any);

			await handlerWithCallbacks.acknowledgeEvent(eventId);

			// Restore setTimeout
			global.setTimeout = originalSetTimeout;

			expect(onRetry).toHaveBeenCalledTimes(2);
			expect(onRetry.mock.calls[0]).toEqual([eventId, 1, expect.any(Error)]);
			expect(onRetry.mock.calls[1]).toEqual([eventId, 2, expect.any(Error)]);
		});
	});
});
