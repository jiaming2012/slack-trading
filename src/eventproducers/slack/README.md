This event producer takes slack API requests and transforms them into events

## Dev
Linked to CryptoHathaway slack channel

### Setting up slack to send commands to application
1. Open endpoint to the internet: `ngrok http 8080`
2. Edit the `Request Url` for each slack command: https://api.slack.com/apps/A03C4E2TA6M/slash-commands? 
   1. Login: google account: jcolecrypto@gmail.com

#### How slack interacts with third party apps
Slack has two different ways to send data to third party apps:
1. Event Subscriptions: the third party app receives a payload from slack when an event is triggered: e.g. - a user mentions another user
2. Slack Commands: each slack command has a request url. The command's payload is sent to the request url.
