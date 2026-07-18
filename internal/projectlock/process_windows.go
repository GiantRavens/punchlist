//go:build windows

package projectlock

// Windows cannot portably probe an arbitrary PID with the standard library.
// Its leases are recovered by the age threshold instead.
func processAlive(pid int) bool { return true }
