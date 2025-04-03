package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"html"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"github.com/drewwillard/rss-aggregator/internal/config"
	"github.com/drewwillard/rss-aggregator/internal/database"
)

func main() {
	configContents, err := config.Read()
	if err != nil {
		fmt.Printf("issue reading config: %v\n", err)
		return
	}
	sessionState := state{config: &configContents}
	db, err := sql.Open("postgres", configContents.DbURL)
	if err != nil {
		fmt.Printf("database error: %v\n", err)
	}
	dbQueries := database.New(db)
	sessionState.db = dbQueries
	sessionCommands := commands{cMap: make(map[string]func(*state, command) error)}
	sessionCommands.register("login", handlerLogin)
	sessionCommands.register("register", handlerRegister)
	sessionCommands.register("reset", handlerReset)
	sessionCommands.register("users", handlerUsers)
	sessionCommands.register("agg", handlerAgg)
	sessionCommands.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	sessionCommands.register("feeds", handlerFeeds)
	sessionCommands.register("follow", middlewareLoggedIn(handlerFollow))
	sessionCommands.register("following", middlewareLoggedIn(handlerFollowing))
	sessionCommands.register("unfollow", middlewareLoggedIn(handlerUnfollow))
	sessionCommands.register("browse", middlewareLoggedIn(handlerBrowse))
	if len(os.Args) < 2 {
		fmt.Println("not enough arguments")
		os.Exit(1)
	}
	sessionCmd := command{
		name: os.Args[1],
		args: os.Args[2:],
	}
	if err := sessionCommands.run(&sessionState, sessionCmd); err != nil {
		fmt.Printf("command error: %v\n", err)
		os.Exit(1)
	}
}

type state struct {
	db     *database.Queries
	config *config.Config
}

type command struct {
	name string
	args []string
}

type commands struct {
	cMap map[string]func(*state, command) error
}

type RSSFeed struct {
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Item        []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

func (c *commands) register(name string, f func(*state, command) error) {
	c.cMap[name] = f
}

func (c *commands) run(s *state, cmd command) error {
	if err := c.cMap[cmd.name](s, cmd); err != nil {
		return err
	}
	return nil
}

func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("no arguments given, requires <username>")
	}
	_, err := s.db.GetUserByName(context.Background(), cmd.args[0])
	if err != nil {
		return fmt.Errorf("user does not exist in database, use register first")
	}
	if err := s.config.SetUser(cmd.args[0]); err != nil {
		return err
	}
	fmt.Printf("User: %s has been set", cmd.args[0])
	return nil
}

func handlerRegister(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("no arguments given, requires <name>")
	}
	existingUser, err := s.db.GetUserByName(context.Background(), cmd.args[0])
	if err == nil && existingUser.Name != "" {
		return fmt.Errorf("user: %v already exists", existingUser.Name)
	}
	user, err := s.db.CreateUser(context.Background(), database.CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      cmd.args[0],
	})
	if err != nil {
		return fmt.Errorf("error creating user: %v", err)
	}
	if err := s.config.SetUser(cmd.args[0]); err != nil {
		return err
	}
	fmt.Printf("Created user %s with ID %s and set as current user\n", user.Name, user.ID)
	return nil
}

func handlerReset(s *state, _ command) error {
	if err := s.db.RemoveUsers(context.Background()); err != nil {
		return err
	}
	fmt.Println("users reset successfully")
	return nil
}

func handlerUsers(s *state, _ command) error {
	allUsers, err := s.db.GetUsers(context.Background())
	if err != nil {
		return err
	}
	for _, user := range allUsers {
		if user.Name == s.config.CurrentUserName {
			fmt.Printf("* %s (current)\n", user.Name)
		} else {
			fmt.Printf("* %s\n", user.Name)
		}
	}
	return nil
}

func handlerAgg(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("no arguments given, requires <name>")
	}
	parsedTime, err := time.ParseDuration(cmd.args[0])
	if err != nil {
		return fmt.Errorf("error parsing time argument: %v", err)
	}
	fmt.Printf("Collecting feeds every %v\n", parsedTime)
	ticker := time.NewTicker(parsedTime)
	for ; ; <-ticker.C {
		scrapeFeeds(s)
	}
}

func handlerAddFeed(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 2 {
		return fmt.Errorf("not enough arguments, requires <name> and <url>")
	}
	feed, err := s.db.CreateFeed(context.Background(), database.CreateFeedParams{
		ID:        int32(uuid.New().ID()),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      cmd.args[0],
		Url:       cmd.args[1],
		UserID:    user.ID,
	})
	if err != nil {
		return fmt.Errorf("issue creating feed: %v", err)
	}
	followRel, err := s.db.GetFeedFollowsForUser(context.Background(), database.GetFeedFollowsForUserParams{
		ID:        int32(uuid.New().ID()),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID:    user.ID,
		FeedID:    feed.ID,
	})
	if err != nil {
		return fmt.Errorf("issue creating follow relationship: %v", err)
	}
	fmt.Printf("Successfully added feed named: %v at url: %v", followRel.UserName, feed.Url)
	return nil
}

func handlerFeeds(s *state, _ command) error {
	allFeeds, err := s.db.GetAllFeedInfo(context.Background())
	if err != nil {
		return err
	}
	for _, feed := range allFeeds {
		fmt.Println("***")
		fmt.Printf("Name: %s\n", feed.Name)
		fmt.Printf("URL: %s\n", feed.Url)
		fmt.Printf("Added by: %s\n", feed.Username)
	}
	return nil
}

func handlerFollow(s *state, cmd command, user database.User) error {
	feedURL, err := s.db.GetFeedFromURL(context.Background(), cmd.args[0])
	if err != nil {
		return fmt.Errorf("can't find that url: %v", err)
	}
	followRel, err := s.db.GetFeedFollowsForUser(context.Background(), database.GetFeedFollowsForUserParams{
		ID:        int32(uuid.New().ID()),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID:    user.ID,
		FeedID:    feedURL.ID,
	})
	if err != nil {
		return fmt.Errorf("issue creating follow relationship: %v", err)
	}
	fmt.Printf("%v is now following the %v feed", followRel.UserName, followRel.FeedName)
	return nil
}

func handlerFollowing(s *state, _ command, user database.User) error {
	usersFeeds, err := s.db.GetFollowsForUserName(context.Background(), user.Name)
	if err != nil {
		return fmt.Errorf("error getting feeds: %v", err)
	}
	fmt.Println("Current user is following:")
	for _, feed := range usersFeeds {
		fmt.Println(feed.Name)
	}
	return nil
}

func handlerUnfollow(s *state, cmd command, user database.User) error {
	feedToUnfollow, err := s.db.GetFeedFromURL(context.Background(), cmd.args[0])
	if err != nil {
		return fmt.Errorf("error finding feed url: %v", err)
	}
	unfollowAction, err := s.db.UnfollowFeed(context.Background(), database.UnfollowFeedParams{
		UserID: user.ID,
		FeedID: feedToUnfollow.ID,
	})
	if err != nil {
		return fmt.Errorf("error removing follow relationship: %v", err)
	}
	fmt.Printf("successfully unfollowed: %v\n", feedToUnfollow.Name)
	fmt.Printf("deleted relationship id: %v\n", unfollowAction.ID)
	return nil
}

func handlerBrowse(s *state, cmd command, user database.User) error {
	postLimit := 2
	if len(cmd.args) > 0 {
		parseAttempt, err := strconv.Atoi(cmd.args[0])
		if err == nil {
			postLimit = parseAttempt
		}
	}
	listOfPosts, err := s.db.GetPostsForUser(context.Background(), database.GetPostsForUserParams{
		ID:    user.ID,
		Limit: int64(postLimit),
	})
	if err != nil {
		return err
	}
	for _, post := range listOfPosts {
		fmt.Println("*****")
		fmt.Println(post.Title)
		fmt.Println(post.Description)
		fmt.Println("-")
		fmt.Printf("Read more: %v\n", post.Url)
		fmt.Println("- - - - - - - - - - - - - - - -")
	}
	return nil
}

func fetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User.Agent", "rssagg")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var feedStruct RSSFeed
	decoder := xml.NewDecoder(res.Body)
	if err := decoder.Decode(&feedStruct); err != nil {
		return nil, err
	}
	feedStruct.Channel.Title = html.UnescapeString(feedStruct.Channel.Title)
	feedStruct.Channel.Description = html.UnescapeString(feedStruct.Channel.Description)
	for _, item := range feedStruct.Channel.Item {
		item.Title = html.UnescapeString(item.Title)
		item.Description = html.UnescapeString(item.Description)
	}
	return &feedStruct, nil
}

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		currentUser, err := s.db.GetUserByName(context.Background(), s.config.CurrentUserName)
		if err != nil {
			return fmt.Errorf("user not logged in: %v", err)
		}
		return handler(s, cmd, currentUser)
	}
}

func scrapeFeeds(s *state) {
	nextFeed, err := s.db.GetNextFeedToFetch(context.Background())
	if err != nil {
		return
	}
	feedMarked, err := s.db.MarkFeedFetched(context.Background(), database.MarkFeedFetchedParams{
		ID:            nextFeed.ID,
		UpdatedAt:     time.Now(),
		LastFetchedAt: sql.NullTime{Time: time.Now(), Valid: true},
	})
	if err != nil {
		return
	}
	fetchedFeed, err := fetchFeed(context.Background(), feedMarked.Url)
	if err != nil {
		return
	}
	fmt.Println("---------------------------------------------")
	fmt.Printf("Now adding posts from: %v\n", feedMarked.Name)
	numAdds := 0
	lastLoggedIssue := ""
	for _, item := range fetchedFeed.Channel.Item {
		postDate, err := time.Parse("02 Jan 2006 15:04:05 -0700", item.PubDate[5:])
		if err != nil {
			lastLoggedIssue = fmt.Sprintf("issue parsing date from post: %v\ncan't parse %v\n", item.Title, item.PubDate[5:])
			continue
		}
		if len(item.Title) < 3 {
			continue
		}
		postAdded, err := s.db.CreatePost(context.Background(), database.CreatePostParams{
			ID:          int32(uuid.New().ID()),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Title:       html.UnescapeString(item.Title),
			Url:         item.Link,
			Description: html.UnescapeString(item.Description),
			PublishedAt: postDate,
			FeedID:      feedMarked.ID,
		})
		if err == nil {
			numAdds++
			continue
		} else {
			if strings.Contains(err.Error(), "url_key") {
				continue
			}
			fmt.Printf("issue adding %v to database: %v\n", postAdded.Title, err)
		}
	}
	fmt.Println("***")
	fmt.Printf("%v posts successfully added!!\n", numAdds)
	fmt.Println("***")
	if lastLoggedIssue != "" {
		fmt.Printf("Possible problem: %v", lastLoggedIssue)
	} else {
		fmt.Println("No issues :)")
	}
}
