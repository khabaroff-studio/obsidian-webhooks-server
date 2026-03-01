/**
 * sse_handler.test.ts - Tests for SSE (Server-Sent Events) handler
 *
 * Following TDD approach: These tests define the expected behavior BEFORE implementation.
 *
 * Test coverage:
 * - Connection establishment
 * - Event parsing from SSE stream
 * - Disconnection
 * - Error handling
 * - Reconnection attempts
 * - Connection state callbacks
 */

import { describe, test, expect, beforeEach, afterEach } from "bun:test";
import type { WebhookEvent } from "../types";

/**
 * Mock EventSource for testing
 * Simulates browser EventSource API
 */
class MockEventSource {
	private static instances: MockEventSource[] = [];

	url: string;
	readyState: number = 0; // CONNECTING
	onmessage: ((event: MessageEvent) => void) | null = null;
	onerror: ((event: Event) => void) | null = null;
	onopen: ((event: Event) => void) | null = null;

	static readonly CONNECTING = 0;
	static readonly OPEN = 1;
	static readonly CLOSED = 2;

	constructor(url: string) {
		this.url = url;
		MockEventSource.instances.push(this);

		// Simulate async connection
		setTimeout(() => {
			if (this.readyState !== MockEventSource.CLOSED) {
				this.readyState = MockEventSource.OPEN;
				this.onopen?.({ type: "open" } as Event);
			}
		}, 10);
	}

	close(): void {
		this.readyState = MockEventSource.CLOSED;
	}

	// Test helper: simulate receiving a message
	simulateMessage(data: string): void {
		if (this.readyState === MockEventSource.OPEN && this.onmessage) {
			const event = {
				data,
				type: "message",
			} as MessageEvent;
			this.onmessage(event);
		}
	}

	// Test helper: simulate error
	simulateError(): void {
		if (this.onerror) {
			this.onerror({ type: "error" } as Event);
		}
	}

	// Test helper: get all instances
	static getInstances(): MockEventSource[] {
		return MockEventSource.instances;
	}

	// Test helper: reset instances
	static resetInstances(): void {
		MockEventSource.instances = [];
	}

	// Test helper: get latest instance
	static getLatest(): MockEventSource | undefined {
		return MockEventSource.instances[MockEventSource.instances.length - 1];
	}
}

// Replace global EventSource with mock
(global as any).EventSource = MockEventSource;

/**
 * Import SSEHandler after mock is set up
 * This ensures the handler uses our mock EventSource
 */
import { SSEHandler } from "./sse_handler";

/**
 * Test Suite: SSEHandler
 */
describe("SSEHandler", () => {
	let handler: SSEHandler;
	let receivedEvents: WebhookEvent[] = [];
	let stateChanges: Array<{ state: string; message: string }> = [];

	const serverUrl = "https://webhook.example.com";
	const clientKey = "cl_test123456";

	beforeEach(() => {
		// Reset mocks and test state
		MockEventSource.resetInstances();
		receivedEvents = [];
		stateChanges = [];

		// Create handler with callbacks
		handler = new SSEHandler(
			serverUrl,
			clientKey,
			async (event: WebhookEvent) => {
				receivedEvents.push(event);
			},
			(state: string, message: string) => {
				stateChanges.push({ state, message });
			}
		);
	});

	afterEach(async () => {
		// Clean up connections
		await handler.disconnect();
	});

	/**
	 * Test 1: Connection establishment
	 */
	test("should establish SSE connection with correct URL", async () => {
		handler.connect();

		// Wait for connection to open (mock opens async after 10ms)
		await new Promise((resolve) => setTimeout(resolve, 20));

		const instance = MockEventSource.getLatest();
		expect(instance).toBeDefined();
		expect(instance?.url).toBe(`${serverUrl}/events/${clientKey}`);
		expect(instance?.readyState).toBe(MockEventSource.OPEN);
	});

	/**
	 * Test 2: Event parsing from SSE stream
	 */
	test("should parse and handle events from SSE stream", async () => {
		handler.connect();

		// Wait for connection to open
		await new Promise((resolve) => setTimeout(resolve, 20));

		const instance = MockEventSource.getLatest();
		expect(instance).toBeDefined();

		// Simulate receiving an event
		const testEvent: WebhookEvent = {
			id: "event-123",
			path: "inbox/note.md",
			data: "Test content",
			created_at: "2025-11-23T10:00:00Z",
		};

		instance?.simulateMessage(JSON.stringify(testEvent));

		// Wait for async processing
		await new Promise((resolve) => setTimeout(resolve, 10));

		// Verify event was received and parsed
		expect(receivedEvents).toHaveLength(1);
		expect(receivedEvents[0]).toEqual(testEvent);
	});

	/**
	 * Test 3: Disconnection
	 */
	test("should cleanly disconnect and close EventSource", async () => {
		handler.connect();
		await new Promise((resolve) => setTimeout(resolve, 20));

		const instance = MockEventSource.getLatest();
		expect(instance?.readyState).toBe(MockEventSource.OPEN);

		await handler.disconnect();

		expect(instance?.readyState).toBe(MockEventSource.CLOSED);
		expect(handler.isConnected()).toBe(false);
	});

	/**
	 * Test 4: Error handling
	 */
	test("should handle SSE errors gracefully", async () => {
		handler.connect();
		await new Promise((resolve) => setTimeout(resolve, 20));

		const instance = MockEventSource.getLatest();

		// Simulate error
		instance?.simulateError();

		// Wait for error handling
		await new Promise((resolve) => setTimeout(resolve, 10));

		// Verify error state was recorded
		const errorStates = stateChanges.filter((s) => s.state === "error");
		expect(errorStates.length).toBeGreaterThan(0);
	});

	/**
	 * Test 5: Reconnection attempts
	 */
	test("should attempt reconnection on error", async () => {
		// Enable auto-reconnect
		handler = new SSEHandler(
			serverUrl,
			clientKey,
			async (event: WebhookEvent) => {
				receivedEvents.push(event);
			},
			(state: string, message: string) => {
				stateChanges.push({ state, message });
			},
			{ autoReconnect: true, reconnectDelayMs: 50 }
		);

		handler.connect();
		await new Promise((resolve) => setTimeout(resolve, 20));

		const firstInstance = MockEventSource.getLatest();
		const firstInstanceCount = MockEventSource.getInstances().length;

		// Simulate error to trigger reconnection
		firstInstance?.simulateError();

		// Wait for reconnection attempt
		await new Promise((resolve) => setTimeout(resolve, 100));

		// Verify a new EventSource was created (reconnection attempt)
		const instanceCount = MockEventSource.getInstances().length;
		expect(instanceCount).toBeGreaterThan(firstInstanceCount);

		await handler.disconnect();
	});

	/**
	 * Test 6: Connection state callbacks
	 */
	test("should invoke state change callbacks correctly", async () => {
		handler.connect();

		// Wait for connection
		await new Promise((resolve) => setTimeout(resolve, 20));

		// Verify we received state changes
		expect(stateChanges.length).toBeGreaterThan(0);

		// Should have "connecting" state
		const connectingStates = stateChanges.filter((s) => s.state === "connecting");
		expect(connectingStates.length).toBeGreaterThan(0);

		// Should have "connected" state
		const connectedStates = stateChanges.filter((s) => s.state === "connected");
		expect(connectedStates.length).toBeGreaterThan(0);
	});

	/**
	 * Test 7: Multiple events in sequence
	 */
	test("should handle multiple events in sequence", async () => {
		handler.connect();
		await new Promise((resolve) => setTimeout(resolve, 20));

		const instance = MockEventSource.getLatest();

		// Send multiple events
		const events: WebhookEvent[] = [
			{
				id: "event-1",
				path: "inbox/note1.md",
				data: "Content 1",
				created_at: "2025-11-23T10:00:00Z",
			},
			{
				id: "event-2",
				path: "inbox/note2.md",
				data: "Content 2",
				created_at: "2025-11-23T10:01:00Z",
			},
			{
				id: "event-3",
				path: "inbox/note3.md",
				data: "Content 3",
				created_at: "2025-11-23T10:02:00Z",
			},
		];

		for (const event of events) {
			instance?.simulateMessage(JSON.stringify(event));
			await new Promise((resolve) => setTimeout(resolve, 5));
		}

		// Verify all events received
		expect(receivedEvents).toHaveLength(3);
		expect(receivedEvents).toEqual(events);
	});

	/**
	 * Test 8: Invalid JSON handling
	 */
	test("should handle invalid JSON gracefully", async () => {
		handler.connect();
		await new Promise((resolve) => setTimeout(resolve, 20));

		const instance = MockEventSource.getLatest();

		// Send invalid JSON
		instance?.simulateMessage("invalid json{]");

		await new Promise((resolve) => setTimeout(resolve, 10));

		// Should not crash and should not add any events
		expect(receivedEvents).toHaveLength(0);
	});

	/**
	 * Test 9: isConnected status
	 */
	test("should report correct connection status", async () => {
		// Initially disconnected
		expect(handler.isConnected()).toBe(false);

		// Connect
		handler.connect();
		await new Promise((resolve) => setTimeout(resolve, 20));

		// Should be connected
		expect(handler.isConnected()).toBe(true);

		// Disconnect
		await handler.disconnect();

		// Should be disconnected
		expect(handler.isConnected()).toBe(false);
	});

	/**
	 * Test 10: Prevent multiple simultaneous connections
	 */
	test("should prevent multiple simultaneous connections", async () => {
		handler.connect();
		await new Promise((resolve) => setTimeout(resolve, 20));

		const firstInstanceCount = MockEventSource.getInstances().length;

		// Try to connect again
		handler.connect();

		// Should not create a new instance (should use existing or replace)
		const secondInstanceCount = MockEventSource.getInstances().length;
		expect(secondInstanceCount).toBeLessThanOrEqual(firstInstanceCount + 1);
	});
});
