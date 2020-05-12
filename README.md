# Pairing Bot :pear::robot:
<a href='http://www.recurse.com' title='Made with love at the Recurse Center'><img src='https://cloud.githubusercontent.com/assets/2883345/11325206/336ea5f4-9150-11e5-9e90-d86ad31993d8.png' height='20px'/></a>

A Zulip bot that partners people for pair programming practice :)

### How to use Pairing Bot as an end-user
Pairing Bot interacts through private messages on [Zulip](https://zulipchat.com/).
* `subscribe` to start getting matched with other Pairing Bot users for pair programming
* `schedule monday wednesday friday` to set your weekly pairing schedule
  * In this example, Pairing Bot has been set to find pairing partners for the user on every Monday, Wednesday, and Friday
  * The user can schedule pairing for any combination of days in the week
* `skip tomorrow` to skip pairing tomorrow
  * This is valid until matches go out at 04:00 UTC
* `unskip tomorrow` to undo skipping tomorrow
* `status` to show your current schedule, skip status, and name
* `unsubscribe` to stop getting matched entirely
  * This removes the user from the database. Since logs are anonymous, after **unsubscribe** Pairing Bot has no record of that user
 
### About Pairing Bot's setup and deployment
 * Serverless. RC's instance is currently deployed on [App Engine](https://cloud.google.com/appengine/docs/standard/)
 * [Firestore database](https://cloud.google.com/firestore/docs/)
 * Deployed on pushes to the `release` branch with [Cloud Build](https://cloud.google.com/cloud-build/docs/)
 * The database must be prepopulated with two pieces of data:  an authentication token (which the bot uses to validate incoming webhooks), and an api key (which the bot uses to send private messages to Zulip users)
 * Zulip has bot types. Pairing Bot is of type `outgoing webhook`
 * Pair programming matches are made, and the people who've been matched are notified, any time an HTTP GET request is issued to `/cron`

### Pull requests are welcome, especially from RC community members!
Pairing Bot is an [RC community project](https://recurse.zulipchat.com/#narrow/stream/198090-rc-community.20software).

**Your contributions are welcome and encouraged, no matter your prior experience!**

Pairing Bot's source code is heavily commented, so that it's easier for others to contribute. Also, Pairing Bot doesn't use any API wrappers or 3rd party libraries (other than those necessary to interface with Google Cloud Platform utilities), in the hope that having her behavior more frankly exposed helps us learn. 
