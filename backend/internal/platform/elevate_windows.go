package platform

// ElevatedCommand resolves how to run the validated helper: directly
// when already elevated, otherwise through the UAC prompt wrapper.
func ElevatedCommand(helper string, args []string) (string, []string) {
	if IsElevated() {
		return helper, args
	}
	return ElevateCommand(helper, args)
}
