# Issue #035: XHR Response Parsed Without Schema Validation

**Severity:** MEDIUM

## Issue

API responses in the UI are parsed with direct property access (`data.field`) without validating the schema. Unexpected server responses (partial JSON, schema changes) cause silent undefined errors rather than surfaced error states.

## Location

`ui/src/hooks/useJsonRpc.ts` or API fetch hooks.

## Fix

Add runtime schema validation (zod or simple type guards) on API responses. Add a test with a malformed response.

## Status

- [ ] TODO
