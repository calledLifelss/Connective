//go:build !windows

package platform

// ElevatedCommand resolves how to run the validated helper: directly
// when already privileged, otherwise through the elevation runner.
func ElevatedCommand(helper string, args []string) (string, []string) {
	if IsElevated() {
		return helper, args
	}
	runner := HelperRunner()
	out := append([]string{}, runner[1:]...)
	out = append(out, helper)
	out = append(out, args...)
	return runner[0], out
}
