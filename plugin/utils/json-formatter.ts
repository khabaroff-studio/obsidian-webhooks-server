/**
 * json-formatter.ts - Automatic JSON to Markdown conversion
 *
 * Detects JSON data and formats it as Markdown with YAML frontmatter
 */

export interface JsonFormatterOptions {
	/** Enable automatic JSON detection and formatting (default: true) */
	enabled?: boolean;
	/** Pretty print JSON if it's not a known format (default: false) */
	prettyPrintUnknown?: boolean;
}

const DEFAULT_OPTIONS: JsonFormatterOptions = {
	enabled: true,
	prettyPrintUnknown: false,
};

/**
 * Format data, auto-detecting and converting JSON to Markdown
 *
 * @param data - Raw data string (possibly JSON)
 * @param options - Formatting options
 * @returns Formatted markdown string
 */
export function formatData(
	data: string,
	options?: JsonFormatterOptions
): string {
	const opts = { ...DEFAULT_OPTIONS, ...options };

	// If formatting is disabled, return as-is
	if (!opts.enabled) {
		return data;
	}

	// Try to parse as JSON
	const parsed = tryParseJSON(data);
	if (!parsed) {
		// Not valid JSON - return as-is
		return data;
	}

	// Check if it's a known JSON structure
	if (isNoteFormat(parsed)) {
		return formatAsNote(parsed);
	}

	// Unknown JSON format - optionally pretty print
	if (opts.prettyPrintUnknown) {
		return "```json\n" + JSON.stringify(parsed, null, 2) + "\n```";
	}

	// Return original data
	return data;
}

/**
 * Try to parse string as JSON
 *
 * @param data - String to parse
 * @returns Parsed object or null if invalid JSON
 */
function tryParseJSON(data: string): unknown {
	try {
		return JSON.parse(data);
	} catch {
		return null;
	}
}

/**
 * Check if JSON object matches note format
 *
 * Expected format:
 * {
 *   "title": "...",
 *   "content": "...",
 *   "tags": [...],
 *   ...other fields
 * }
 */
function isNoteFormat(obj: unknown): obj is Record<string, unknown> {
	return (
		typeof obj === "object" &&
		obj !== null &&
		("title" in obj ||
			"content" in obj ||
			"tags" in obj)
	);
}

/**
 * Format JSON object as Markdown note with YAML frontmatter
 *
 * Extracts known fields:
 * - title: Note title (goes to frontmatter)
 * - content: Note body (goes to markdown body)
 * - tags: Array of tags (goes to frontmatter)
 * - date/created/updated: Timestamps (go to frontmatter)
 * - Any other fields go to frontmatter as-is
 */
function formatAsNote(obj: Record<string, unknown>): string {
	const frontmatter: Record<string, unknown> = {};
	let content = "";

	// Extract content field
	if (obj.content !== undefined) {
		content = typeof obj.content === "string" ? obj.content : JSON.stringify(obj.content);
	}

	// Extract and format known fields for frontmatter
	const knownFields = ["title", "tags", "date", "created", "updated", "author"];
	for (const field of knownFields) {
		if (obj[field] !== undefined) {
			frontmatter[field] = obj[field];
		}
	}

	// Add any other fields to frontmatter (except content)
	for (const key in obj) {
		if (
			!knownFields.includes(key) &&
			key !== "content" &&
			Object.prototype.hasOwnProperty.call(obj, key)
		) {
			frontmatter[key] = obj[key];
		}
	}

	// Build the markdown output
	let output = "";

	// Add frontmatter if there are any fields
	if (Object.keys(frontmatter).length > 0) {
		output += "---\n";
		for (const [key, value] of Object.entries(frontmatter)) {
			output += formatFrontmatterField(key, value);
		}
		output += "---\n\n";
	}

	// Add content
	if (content) {
		output += content;
	}

	return output;
}

/**
 * Format a single frontmatter field as YAML
 */
function formatFrontmatterField(key: string, value: unknown): string {
	if (value === null || value === undefined) {
		return `${key}: null\n`;
	}

	if (Array.isArray(value)) {
		// Format arrays in inline style: [item1, item2]
		const items = value.map((v) => JSON.stringify(v)).join(", ");
		return `${key}: [${items}]\n`;
	}

	if (typeof value === "object") {
		// Format objects as JSON for simplicity
		return `${key}: ${JSON.stringify(value)}\n`;
	}

	if (typeof value === "string") {
		// Quote strings if they contain special characters
		if (value.includes(":") || value.includes("\n") || value.includes("#")) {
			return `${key}: "${value.replace(/"/g, '\\"')}"\n`;
		}
		return `${key}: ${value}\n`;
	}

	// Numbers, booleans, etc. — convert via String() for type safety
	const stringified = typeof value === "number" || typeof value === "boolean"
		? value.toString()
		: JSON.stringify(value);
	return `${key}: ${stringified}\n`;
}
