Notification Ad Pusher
===

This is an ad hoc application for sending push notifications containing advertising.

The app first contacts a push notification service to get a list of subscribers. This list is saved in a database.

The app then contacts an ad vendor to retreive a unique ad for each user. After receipt, each ad is then formatted and sent to the notification service. Form there, it goes out to the users.

Each successful push is recorded in the database so that if the process is interupted, it can be re-started without risk of sending duplicate ads to subscribers who have already gotten one. Each _unseccessful_ push is also recorded, and these are automatically retried.

I used GoLang for this project because of GoLang's ability to perform tasks concurrently. The app makes many thousands of API calls each time it is used, and being able to make the calls concurrently improved its speed immensely vs. single-thread.
