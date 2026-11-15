package git

import "strings"

// redactCredentials removes the userinfo part of a remote URL from a message.
// git echoes the remote URL (including the token) in push/clone errors, and
// those errors are logged and stored in run events.
func redactCredentials(message, remoteURL string) string {
	scheme := strings.Index(remoteURL, "://")
	at := strings.LastIndex(remoteURL, "@")
	if scheme < 0 || at <= scheme {
		return message
	}
	credentials := remoteURL[scheme+3 : at]
	if credentials == "" {
		return message
	}
	return strings.ReplaceAll(message, credentials, "***")
}
