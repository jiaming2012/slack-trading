package eventmodels

// IncomingSlackRequest is received from slack commands. The Content-Type is x-www-form-urlencoded
type IncomingSlackRequest struct {
	ChannelID   string `schema:"channel_id"`
	ChannelName string `schema:"channel_name"`
	Token       string `schema:"token"`
	TeamDomain  string `schema:"team_domain"`
	Username    string `schema:"user_name"`
	Cmd         string `schema:"command"`
	Params      string `schema:"text"`
	ResponseURL string `schema:"response_url"`
	TriggerID   string `schema:"trigger_id"`
}
