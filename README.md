# Pairing Bot :pear::robot:
<a href='http://www.recurse.com' title='Made with love at the Recurse Center'><img src='https://cloud.githubusercontent.com/assets/2883345/11325206/336ea5f4-9150-11e5-9e90-d86ad31993d8.png' height='20px'/></a> [![Go Report Card](https://goreportcard.com/badge/github.com/chrobid/pairing-bot)](https://goreportcard.com/report/github.com/chrobid/pairing-bot)
A Zulip bot that partners people for pair programming practice :)

### How to use Pairing Bot as an end-user
Pairing Bot interacts through private messages on [Zulip](https://zulipchat.com/)
* `subscribe` to start getting matched with other Pairing Bot users for pair programming
* `schedule monday wednesday friday` to set your weekly pairing schedule
  * In this example, Pairing Bot has been set to find pairing partners for the user on every Monday, Wednesday, and Friday
  * The user can schedule pairing for any combination of days in the week
* `skip tomorrow` to skip pairing tomorrow
  * This is valid until matches go out at 8am
  * If the user issues **skip tomorrow** at 4am on Tuesday, they will not be matched for pairing on Tuesday, but they will be matched for pairing on Wednesday (if Wednesday is in their schedule)
* `unskip tomorrow` to undo skipping tomorrow
* `status` to show your current schedule, skip status, and name
* `unsubscribe` to stop getting matched entirely
  * This removes the user from the database. Since logs are anonymous, after **unsubscribe** Pairing Bot has no record of that user
 
 ### About Pairing Bot's setup and deployment
 * Serverless. RC's instance is currently deployed on [App Engine](https://cloud.google.com/appengine/docs/)
 * [Firestore database](https://cloud.google.com/firestore/docs/)
 * Deployed on pushes to master with [Cloud Build](https://cloud.google.com/cloud-build/docs/)
 * The database must be prepopulated with two pieces of data:  an authentication token (which Zulip issues to a bot when their account is created), and an api key
 * Zulip has bot types. Pairing Bot is of type `outgoing webhook`
 * Pair programming matches are made, and the people who've been matched are notified, any time an HTTP `GET` request is issued to `/cron`

