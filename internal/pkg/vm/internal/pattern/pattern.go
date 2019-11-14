package pattern

// IsAccountActor pattern checks if the caller is an account actor.
type IsAccountActor struct {}

// IsMatch returns "True" if the patterns matches
func (IsAccountActor) IsMatch() bool {
	panic("byteme")
}