package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/smcallister/gator/internal/config"
	"github.com/smcallister/gator/internal/database"
	"github.com/smcallister/gator/internal/rss"
)

import _ "github.com/lib/pq"

type state struct {
	db  *database.Queries
	cfg *config.Configuration
}

type command struct {
	Name string
	Args []string
}

type commands struct {
	Handlers map[string]func(*state, command) error
}

func (c *commands) run(s *state, cmd command) error {
	// Get the handler for this command.
	handler, exists := c.Handlers[cmd.Name]
	if !exists {
		return fmt.Errorf("Command %s not found", cmd.Name)
	}

	// Run the handler.
	return handler(s, cmd)
}

func (c *commands) register(name string, f func(*state, command) error) {
	// Add the handler to the list of commands.
	if c.Handlers == nil {
		c.Handlers = make(map[string]func(*state, command) error)
	}

	c.Handlers[name] = f
}

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		// Get the current user.
		user, err := s.db.GetUser(context.Background(), s.cfg.CurrentUserName)
		if err != nil {
			return err
		}

		// Call the handler with the user.
		return handler(s, cmd, user)
	}
}

func handlerLogin(s *state, cmd command) error {
	// Validate arguments.
	if len(cmd.Args) == 0 {
		return fmt.Errorf("Format: login <user name>\n")
	}

	name := cmd.Args[0]

	// Make sure the user exists.
	_, err := s.db.GetUser(context.Background(), name)
	if err != nil {
		return err
	}

	// Set the user in the config file.
	err = s.cfg.SetUser(name)
	if err != nil {
		return err
	}
	
	fmt.Printf("Set user to %s\n", name)
	return nil
}

func handlerRegister(s *state, cmd command) error {
	// Validate arguments.
	if len(cmd.Args) == 0 {
		return fmt.Errorf("Format: register <name>\n")
	}

	// Create the user.
	currentTime := time.Now()
	params := database.CreateUserParams{uuid.New(),
							   			currentTime,
							   			currentTime,
							   			cmd.Args[0]}
	user, err := s.db.CreateUser(context.Background(), params)
	if err != nil {
		return err
	}

	// Set the user in the config file.
	err = s.cfg.SetUser(cmd.Args[0])
	if err != nil {
		return err
	}

	fmt.Printf("%+v\n", user)
	return nil
}

func handlerReset(s *state, cmd command) error {
	// Delete all users.
	return s.db.DeleteAllUsers(context.Background())
}

func handlerGetUsers(s *state, cmd command) error {
	// Get all users.
	users, err := s.db.GetAllUsers(context.Background())
	if err != nil {
		return err
	}

	// Print the users to the console.
	for _, user := range users {
		fmt.Printf("* %s", user.Name)
		if user.Name == s.cfg.CurrentUserName {
			fmt.Printf(" (current)")
		}

		fmt.Printf("\n")
	}

	return nil
}

func handlerAgg(s *state, cmd command) error {
	// Get the feed.
	feed, err := rss.FetchFeed(context.Background(), "https://www.wagslane.dev/index.xml")
	if err != nil {
		return err
	}

	// Print the feed.
	fmt.Printf("%+v\n", feed)
	return nil
}

func handlerAddFeed(s *state, cmd command, user database.User) error {
	// Validate arguments.
	if len(cmd.Args) < 2 {
		return fmt.Errorf("Format: addfeed <name> <url>\n")
	}

	name := cmd.Args[0]
	url := cmd.Args[1]
	
	// Create the feed.
	currentTime := time.Now()
	feedParams := database.CreateFeedParams{uuid.New(),
							   			    currentTime,
							   			    currentTime,
							   			    name,
										    url,
										    user.ID}
	feed, err := s.db.CreateFeed(context.Background(), feedParams)
	if err != nil {
		return err
	}

	// Add a feed follow.
	feedFollowParams := database.CreateFeedFollowParams{uuid.New(),
							   			                currentTime,
							   				  			currentTime,
							   				  			user.ID,
											  			feed.ID}
	_, err = s.db.CreateFeedFollow(context.Background(), feedFollowParams)
	if err != nil {
		return err
	}

	fmt.Printf("%+v\n", feed)
	return nil
}

func handlerGetFeeds(s *state, cmd command) error {
	// Get all feeds.
	feeds, err := s.db.GetAllFeeds(context.Background())
	if err != nil {
		return err
	}

	// Print the feeds to the console.
	for _, feed := range feeds {
		user, err := s.db.GetUserByID(context.Background(), feed.UserID)
		if err != nil {
			return err
		}

		fmt.Printf("Name: \"%s\", URL: \"%s\", User: \"%s\"\n",
				   feed.Name,
				   feed.Url,
				   user.Name)
	}

	return nil
}

func handlerFollow(s *state, cmd command, user database.User) error {
	// Validate arguments.
	if len(cmd.Args) == 0 {
		return fmt.Errorf("Format: follow <url>\n")
	}

	url := cmd.Args[0]

	// Get the feed.
	feed, err := s.db.GetFeedByUrl(context.Background(), url)
	if err != nil {
		return err
	}

	// Create the feed follow.
	currentTime := time.Now()
	params := database.CreateFeedFollowParams{uuid.New(),
							   			      currentTime,
							   				  currentTime,
							   				  user.ID,
											  feed.ID}
	feedFollow, err := s.db.CreateFeedFollow(context.Background(), params)
	if err != nil {
		return err
	}

	fmt.Printf("%s is now following \"%s\"\n", feedFollow.UserName, feedFollow.FeedName)
	return nil
}

func handlerFollowing(s *state, cmd command, user database.User) error {
	// Get all feed follows for the current user.
	feedFollows, err := s.db.GetFeedFollowsForUser(context.Background(), user.ID)
	if err != nil {
		return err
	}

	// Print the feed follows to the console.
	for _, feedFollow := range feedFollows {
		fmt.Printf("%s\n", feedFollow.FeedName)
	}

	return nil
}

func handlerUnfollow(s *state, cmd command, user database.User) error {
	// Validate arguments.
	if len(cmd.Args) == 0 {
		return fmt.Errorf("Format: unfollow <url>\n")
	}

	url := cmd.Args[0]

	// Delete the feed follow.
	params := database.DeleteFeedFollowParams{user.ID, url}
	err := s.db.DeleteFeedFollow(context.Background(), params)
	if err != nil {
		return err
	}

	fmt.Printf("%s has unfollowed \"%s\"\n", user.Name, url)
	return nil
}

func main() {
	// Generate the state.
	c, err := config.Read()
	if err != nil {
		fmt.Printf("Error reading config file: %v\n", err)
		os.Exit(1)
	}

	var s state
	s.cfg = c

	// Register all commands.
	var cmds commands
	cmds.register("login", handlerLogin)
	cmds.register("register", handlerRegister)
	cmds.register("reset", handlerReset)
	cmds.register("users", handlerGetUsers)
	cmds.register("agg", handlerAgg)
	cmds.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	cmds.register("feeds", handlerGetFeeds)
	cmds.register("follow", middlewareLoggedIn(handlerFollow))
	cmds.register("following", middlewareLoggedIn(handlerFollowing))
	cmds.register("unfollow", middlewareLoggedIn(handlerUnfollow))

	// Connect to the database.
	db, err := sql.Open("postgres", c.DBUrl)
	if err != nil {
		fmt.Printf("Failed to connect to the database: %v\n", err)
		os.Exit(1)
	}

	s.db = database.New(db)

	// Run the given command.
	if len(os.Args) < 2 {
		fmt.Printf("No command provided\n")
		os.Exit(1)
	}

	var cmd command
	cmd.Name = os.Args[1]
	cmd.Args = os.Args[2:]
	err = cmds.run(&s, cmd)
	if err != nil {
		fmt.Printf("%s failed: %v", cmd.Name, err)
		os.Exit(1)
	}
}