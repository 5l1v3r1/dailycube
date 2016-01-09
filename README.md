# Daily Cube

This automatically posts a "scramble of the day" to a Facebook page. It is useful for Rubik's cube groups, where everybody does one solve each day and posts their time.

# Usage

First and foremost, you must have the Go programming language installed and configured. Once you `go get` this repository, you can generate a configuration file as follows and enter your own information:

    $ go run genconfig/main.go output.json
    Admin password: ***
    FB App ID: ***
    FB Secret: *****
    Landing URL (e.g. http://foo.com): http://localhost:1337
    Time zone (e.g. UTC, America/New_York): America/New_York

Once you have the configuration file, run the main server as follows:

    $ go run *.go <port>

Navigate to the server in your browser and go through the setup steps. Once the server is hooked up to your Facebook account and knows about the group URL, it will automatically post a scramble to that group once a day.
