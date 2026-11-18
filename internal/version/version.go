package version

// Version is set at release time with -ldflags; "go install" builds read it from the module info.
var Version = "dev"
