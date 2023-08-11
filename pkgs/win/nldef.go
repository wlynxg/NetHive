package win

type NLDadState uint32

const (
	NldsInvalid NLDadState = iota
	NldsTentative
	NldsDuplicate
	NldsDeprecated
	NldsPreferred
)
