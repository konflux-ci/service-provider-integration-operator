package oauth

// Constants that are used both in the operator and OAuth service
// Placed here to keep only a reference to operator packages in OAuth service and not vice versa.
var (
	CallBackRoutePath     = "/oauth/callback"
	AuthenticateRoutePath = "/oauth/authenticate"
)
