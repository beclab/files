package encoder

import "strconv"

// buildFFProbeArgs assembles the argv passed to the ffprobe binary
// for GetMediaInfoInternal. Each argument is a discrete element so
// the receiving process gets the value verbatim - no field-splitting,
// no shell evaluation. Callers must pass inputPath as already
// validated (see PlayPath sanitization at the controller layer); this
// helper does NOT attempt to filter shell metacharacters because
// none are interpreted along the argv path.
//
// httpProtocol controls whether the optional -headers argument is
// emitted; when false the headers value is dropped even if present,
// matching the previous behavior where it was only forwarded for the
// HTTP transport.
func buildFFProbeArgs(inputPath, headers string, threads int, extractChapters, httpProtocol bool) []string {
	args := make([]string, 0, 16)
	args = append(args,
		"-v", "warning",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
	)
	if extractChapters {
		args = append(args, "-show_chapters")
	}
	if httpProtocol && headers != "" {
		args = append(args, "-headers", headers)
	}
	args = append(args,
		"-i", inputPath,
		"-threads", strconv.Itoa(threads),
	)
	return args
}
