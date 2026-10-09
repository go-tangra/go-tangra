// Package preflight checks a module's configuration and environment before
// the service starts and reports every problem at once: a module builds a
// list of Checks (its own plus the reusable ones here: files, TCP and HTTPS
// reachability, the enrolment token), Run executes them and the Report is
// printed as a checklist or as JSON. A module exposes it as
// `<svc> preflight -config <file> [-json]` through Main.
//
// Checks only look: they never start listeners, keep state (DirWritable
// removes its probe file), consume the enrolment token or report secret
// values.
package preflight
