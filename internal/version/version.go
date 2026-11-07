// Package version exposes the public CryptoLink release tag, in x.y form: minor
// updates go 2.0, 2.1, 2.2 and x.0 marks a major update. Bumped manually on each
// tagged release. Lives in
// its own package so any internal package can import it without creating an
// import cycle through the cmd or app packages.
package version

const Release = "2.2"
