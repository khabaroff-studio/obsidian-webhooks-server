/**
 * json-formatter.test.ts - Tests for JSON to Markdown formatter
 */

import { describe, it, expect } from "bun:test";
import { formatData } from "./json-formatter";

describe("JSON Formatter", () => {
	describe("formatData", () => {
		it("should format note with title, content, and tags", () => {
			const input = JSON.stringify({
				title: "My Note",
				content: "Note content here",
				tags: ["tag1", "tag2"],
			});

			const output = formatData(input);

			expect(output).toContain("---");
			expect(output).toContain("title: My Note");
			expect(output).toContain('tags: ["tag1", "tag2"]');
			expect(output).toContain("Note content here");
		});

		it("should handle content-only JSON", () => {
			const input = JSON.stringify({
				content: "Just some content",
			});

			const output = formatData(input);

			expect(output).toContain("Just some content");
			expect(output).not.toContain("---"); // No frontmatter if only content
		});

		it("should handle title-only JSON", () => {
			const input = JSON.stringify({
				title: "My Title",
			});

			const output = formatData(input);

			expect(output).toContain("---");
			expect(output).toContain("title: My Title");
		});

		it("should handle extra fields", () => {
			const input = JSON.stringify({
				title: "Note",
				content: "Content",
				author: "John Doe",
				customField: "custom value",
			});

			const output = formatData(input);

			expect(output).toContain("author: John Doe");
			expect(output).toContain("customField: custom value");
		});

		it("should handle date fields", () => {
			const input = JSON.stringify({
				title: "Note",
				date: "2026-02-01",
				created: "2026-02-01T10:00:00Z",
			});

			const output = formatData(input);

			expect(output).toContain("date: 2026-02-01");
			// Timestamps with colons are quoted in YAML
			expect(output).toContain('created: "2026-02-01T10:00:00Z"');
		});

		it("should return non-JSON data as-is", () => {
			const input = "This is plain text, not JSON";
			const output = formatData(input);

			expect(output).toBe(input);
		});

		it("should handle invalid JSON", () => {
			const input = '{"invalid": json}';
			const output = formatData(input);

			expect(output).toBe(input);
		});

		it("should handle JSON without known fields", () => {
			const input = JSON.stringify({
				foo: "bar",
				baz: 123,
			});

			const output = formatData(input, { prettyPrintUnknown: false });

			// Should return original since no known fields
			expect(output).toBe(input);
		});

		it("should pretty print unknown JSON when enabled", () => {
			const input = JSON.stringify({
				foo: "bar",
				baz: 123,
			});

			const output = formatData(input, { prettyPrintUnknown: true });

			expect(output).toContain("```json");
			expect(output).toContain('"foo": "bar"');
			expect(output).toContain('"baz": 123');
		});

		it("should handle array tags", () => {
			const input = JSON.stringify({
				title: "Note",
				tags: ["work", "important", "review"],
			});

			const output = formatData(input);

			expect(output).toContain('tags: ["work", "important", "review"]');
		});

		it("should handle nested objects in frontmatter", () => {
			const input = JSON.stringify({
				title: "Note",
				metadata: {
					source: "webhook",
					version: 1,
				},
			});

			const output = formatData(input);

			expect(output).toContain('metadata: {"source":"webhook","version":1}');
		});

		it("should quote strings with special characters", () => {
			const input = JSON.stringify({
				title: "Note: With Colon",
				content: "Content here",
			});

			const output = formatData(input);

			expect(output).toContain('title: "Note: With Colon"');
		});

		it("should handle multiline content", () => {
			const input = JSON.stringify({
				title: "Note",
				content: "Line 1\nLine 2\nLine 3",
			});

			const output = formatData(input);

			expect(output).toContain("Line 1\nLine 2\nLine 3");
		});

		it("should be disabled when enabled=false", () => {
			const input = JSON.stringify({
				title: "Note",
				content: "Content",
			});

			const output = formatData(input, { enabled: false });

			expect(output).toBe(input);
		});
	});
});
