package api

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// now0 returns the current time truncated to whole seconds, so that comparing it against a
// value that round-tripped through Unix (seconds-only) timestamps never fails on a sub-second
// difference that has nothing to do with the behaviour under test
func now0() time.Time {
	return time.Now().Truncate(time.Second)
}

// TestResolveAlertReceivedAtZeroMeansNow covers a request that omitted receivedAt entirely (the
// json field defaults to 0)
func TestResolveAlertReceivedAtZeroMeansNow(t *testing.T) {
	now := now0()
	assert.Equal(t, now, resolveAlertReceivedAt(0, now))
}

// TestResolveAlertReceivedAtWithinWindowIsKept covers an ordinary, recent timestamp
func TestResolveAlertReceivedAtWithinWindowIsKept(t *testing.T) {
	now := now0()
	receivedAtUnix := now.Add(-time.Hour).Unix()
	assert.Equal(t, time.Unix(receivedAtUnix, 0), resolveAlertReceivedAt(receivedAtUnix, now))
}

// TestResolveAlertReceivedAtFutureBoundaryIsKept and TestResolveAlertReceivedAtTooFarFutureFallsBackToNow
// cover the "> now+5min" edge: exactly now+5min is accepted, one second past it is not
func TestResolveAlertReceivedAtFutureBoundaryIsKept(t *testing.T) {
	now := now0()
	receivedAtUnix := now.Add(5 * time.Minute).Unix()
	assert.Equal(t, time.Unix(receivedAtUnix, 0), resolveAlertReceivedAt(receivedAtUnix, now))
}

func TestResolveAlertReceivedAtTooFarFutureFallsBackToNow(t *testing.T) {
	now := now0()
	receivedAtUnix := now.Add(5*time.Minute + time.Second).Unix()
	assert.Equal(t, now, resolveAlertReceivedAt(receivedAtUnix, now))
}

// TestResolveAlertReceivedAtPastBoundaryIsKept and TestResolveAlertReceivedAtTooFarPastFallsBackToNow
// cover the "< now-30days" edge: exactly now-30days is accepted, one second before it is not
func TestResolveAlertReceivedAtPastBoundaryIsKept(t *testing.T) {
	now := now0()
	receivedAtUnix := now.Add(-30 * 24 * time.Hour).Unix()
	assert.Equal(t, time.Unix(receivedAtUnix, 0), resolveAlertReceivedAt(receivedAtUnix, now))
}

func TestResolveAlertReceivedAtTooFarPastFallsBackToNow(t *testing.T) {
	now := now0()
	receivedAtUnix := now.Add(-30*24*time.Hour - time.Second).Unix()
	assert.Equal(t, now, resolveAlertReceivedAt(receivedAtUnix, now))
}

// TestResolveAlertReceivedAtMillisecondsInsteadOfSecondsFallsBackToNow is the regression case
// from review finding I1: a Shortcut that accidentally sends milliseconds since epoch (instead
// of seconds) would otherwise be interpreted as a date around the year 57000
func TestResolveAlertReceivedAtMillisecondsInsteadOfSecondsFallsBackToNow(t *testing.T) {
	now := now0()
	millisecondsSinceEpoch := now.UnixMilli()
	assert.Equal(t, now, resolveAlertReceivedAt(millisecondsSinceEpoch, now))
}

// TestResolveAlertReceivedAtNearInt64MaxDoesNotPanic covers the other half of I1: a near-int64-max
// value must never panic or overflow into something that slips past the window check
func TestResolveAlertReceivedAtNearInt64MaxDoesNotPanic(t *testing.T) {
	now := now0()
	assert.Equal(t, now, resolveAlertReceivedAt(math.MaxInt64, now))
}

// TestResolveAlertReceivedAtNegativeFallsBackToNow covers a negative value (before the unix
// epoch), which is also well outside the accepted window
func TestResolveAlertReceivedAtNegativeFallsBackToNow(t *testing.T) {
	now := now0()
	assert.Equal(t, now, resolveAlertReceivedAt(-1, now))
}
