// Package version exposes the public CryptoLink release tag. Single-decimal
// scheme: 1.0, 2.0, 3.0, … bumped manually on each tagged release. Lives in
// its own package so any internal package can import it without creating an
// import cycle through the cmd or app packages.
package version

const Release = "1.0"
