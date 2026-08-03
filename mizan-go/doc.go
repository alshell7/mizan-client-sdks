// Package mizan provides a server-side client for the Mizan billing and metering API.
//
// The client never calculates prices, tax, eligibility, budgets, or credit allocation.
// ExactAmount values are base-10 integer strings: money is halala and Azeer Units are
// milliunits. Mutation retries preserve the original encoded body and idempotency key.
// If TransportError is returned, use its IdempotencyKey with the identical request to
// resolve the unknown outcome safely.
package mizan
