**Simple RSS Blog Aggregator for the terminal.**

Postgres and Go are required on your system before installing.

Install using:

```go install https://github.com/drewwillard/rss-aggregator```

--

The package expects a ```.rssaggconfig.json``` file in the users home directory with the following:

```{"db_url":<local_postgress_db_address>,"current_user_name":""}```

**Some Commands to Get Started**
-```register <username>``` adds user, sets them as logged in
-```addfeed <name> <url>``` adds feed to database
-```agg <time_interval>``` "1m" will scrape the oldest feed every 60 seconds and add any new posts to database
-```browse <number_of_posts>``` prints newest posts
-```login <existing_username>``` sets current user
-```follow <feed_url>``` follow a feed added by another user
-```folowing``` see all feeds current user is following
