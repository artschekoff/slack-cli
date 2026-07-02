package credentials

// SetTestCryptoParams replaces the argon2id parameters with values that are
// fast enough for unit tests. Call this at the top of test files that exercise
// encryption so the test suite doesn't spend seconds on key derivation.
func SetTestCryptoParams() {
	activeParams = cryptoParams{Memory: 64, Time: 1, Threads: 1}
}

// ResetCryptoParams restores the production argon2id parameters.
func ResetCryptoParams() {
	activeParams = productionParams
}
