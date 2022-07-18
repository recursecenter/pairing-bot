# Pairing Bot :pear::robot:
<a href='http://www.recurse.com' title='Made with love at the Recurse Center'><img src='https://cloud.githubusercontent.com/assets/2883345/11325206/336ea5f4-9150-11e5-9e90-d86ad31993d8.png' height='20px'/></a>

A Zulip bot that partners people for pair programming practice :)

### Information for Pairing Bot users
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
 
### Information for Pairing Bot admins
 * Runs in [GCP](https://cloud.google.com/) on [App Engine](https://cloud.google.com/appengine/docs/standard/)
 * Uses [Firestore](https://cloud.google.com/firestore/docs/) for its database
 * Deployed on pushes to the `main` branch with [Cloud Build](https://cloud.google.com/cloud-build/docs/)
 * The database must be prepopulated with two pieces of data:  an authentication token (used to validate incoming requests from Zulip), and an API key (used to talk to the Zulip API).
 * Zulip bots must have an owner set in Zulip. They may only have one owner at a time. RC Pairing Bot's ownership is given to whoever is working on Pairing Bot at the moment. The current owner is [Robert Xu](https://github.com/RobertXu).
 * Onboarding, offboarding, and daily pairing matches are all controlled with cron jobs set in [Cloud Scheduler](https://cloud.google.com/scheduler).
 * Pairing bot has a dev environment where you can test out changes before applying them to the main branch of pairing bot.

### Pull requests are welcome, especially from RC community members!
Pairing Bot is an [RC community project](https://recurse.zulipchat.com/#narrow/stream/198090-rc-community.20software).

**Your contributions are welcome and encouraged, no matter your prior experience!**
