// Package mailvisibility owns the message selection shared by inbox and reader.
package mailvisibility

// SQL is a predicate over an unqualified labels column (or a query with one
// messages alias). Archived messages remain visible; drafts, trash and spam do not.
const SQL = `NOT (labels && ARRAY['DRAFT','TRASH','SPAM']::text[])`
