# Problem types

Every 4xx and 5xx answer is an RFC 9457 document with the media type `application/problem+json` and the members `type`, `title`, `status` and `detail`. `type` is `PROBLEM_BASE_URL` (default `https://geo-service.example/problems/`) plus the name below. Reasons and sources: `docs/api-design-notes.md`, A8 and A9. The schemas and one example per status are in `api/openapi.yaml`. If this file and the notes disagree, the notes win. Any 4xx or 5xx is the client's "error" state.

| Name | Status | Title | Detail | When |
|---|---|---|---|---|
| `malformed-request` | 400 | Malformed request | The request body is not valid JSON. | Empty body, text, or data after the JSON value |
| `invalid-request` | 422 | Invalid request | The request body is missing required fields or has invalid values. | JSON is not an object, a required field is missing or invalid, or the list is empty |
| `request-too-large` | 413 | Request too large | The request body exceeds the maximum allowed size of 16384 bytes. | Body above `MAX_REQUEST_BODY_BYTES` (the number in the text is the configured cap) |
| `unsupported-media-type` | 415 | Unsupported media type | The request Content-Type must be application/json. | Content-Type missing or not `application/json` (a charset parameter is fine) |
| `method-not-allowed` | 405 | Method not allowed | This resource only supports POST. | Not POST on `/v1/ip-checks`; the `Allow` header is `POST` |
| `method-not-allowed` | 405 | Method not allowed | This resource only supports GET and HEAD. | Not GET or HEAD on `/livez`, `/readyz`, `/version`, `/metrics`; the `Allow` header is `GET, HEAD` |
| `database-unavailable` | 503 | Service unavailable | The geolocation database is not loaded yet; retry later. | Valid check request, or `/readyz`, while no database is loaded |
| `internal-error` | 500 | Internal error | An unexpected error occurred. | A lookup error or a panic; the body never holds internals |

Unknown paths get the standard library 404, which is not a problem document.

## `invalid-request`

Only this type has `invalid_params`, a list of `{pointer, reason}` (JSON Pointer, RFC 6901; no codes). All problems of one request are listed together: `ip_address` first, then `allowed_countries` by ascending index. A body that is not a JSON object has the single pointer `""`. A country entry in a reason is quoted as the caller sent it; the IP address is never quoted.

| Pointer | Reason |
|---|---|
| `""` | The request body must be a JSON object. |
| `/ip_address` | ip_address is required. |
| `/ip_address` | ip_address must be a string. |
| `/ip_address` | ip_address must be a valid IPv4 or IPv6 address. |
| `/allowed_countries` | allowed_countries is required. |
| `/allowed_countries` | allowed_countries must be an array of strings. |
| `/allowed_countries` | allowed_countries must contain at least one country code. |
| `/allowed_countries/<index>` | Each entry in allowed_countries must be a string. |
| `/allowed_countries/<index>` | 'USA' is not an ISO 3166-1 alpha-2 country code. (the entry as sent) |

Example:

```json
{
  "type": "https://geo-service.example/problems/invalid-request",
  "title": "Invalid request",
  "status": 422,
  "detail": "The request body is missing required fields or has invalid values.",
  "invalid_params": [
    {"pointer": "/ip_address", "reason": "ip_address must be a valid IPv4 or IPv6 address."},
    {"pointer": "/allowed_countries/0", "reason": "'USA' is not an ISO 3166-1 alpha-2 country code."}
  ]
}
```

## Warnings are not problems

A request that can be answered returns 200. Values that were accepted but not used come back in `warnings` (`duplicate_country`, `invalid_session_id`, `invalid_requested_at`); see A7.
