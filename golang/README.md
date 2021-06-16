# covidwa-scrapers/golang

## Dependencies

Dependencies are managed by go modules

## Install and run

#### Installation
```shell
export API_SECRETWA='2cb522b6-ebd8-3488-a567-de9f238588a3' #secret for Test environment
export API_HOSTWA='covidwa-backend-test.herokuapp.com' #host for Test environment
git clone git@github.com:CovidWA/covidwa-scrapers.git
cd ./covidwa-scrapers/golang
go build ./cmd/covidwa-scrapers-go
go install ./... # This step optionally installs the binary if you have $GOPATH setup
```

#### No-code scraper tutorial, from scratch

_1._ Install Go 1.16.x, you can download the appropriate package for your platform [here](https://golang.org/dl/), and install using instructions [here](https://golang.org/doc/install).

_2._ Follow the installation steps above to download, build and install the go scraper.

_3._ Open `covidwa-scrapers.yaml`.  I prefer to use [sublime text](https://www.sublimetext.com) which has built-in yaml highlighting.

_4._ There's alot of stuff in there, but don't worry about breaking things, we have tests setup to catch simple syntax errors and you won't break the other scrapers if your configuration doesn't work.

_5._ Find the configuration named `acme_test`, this is going to be your template.  Note that the `type` field is `"standard_regexp"`.  While there are many types of scrapers this is the most useful for basic long tail sites.  It's essentially a curl call + a grep call, coded in golang and configurable via yaml.

_6._ Copy & paste the `acme_test` section, now you have a duplicate scraper.

_7._ Rename the duplicate you just created to something else, say `acme_test_2`

_8._ Create a new row in airtable (table sites-dev) for your new test scraper.  Set the key to something unique, say `acme_yourname`.

_9._ Set the `api_key` parameter value to match what you created in airtable.

_10._ Change the url to what you need to scrape, let's try `https://www.google.com/`

_11._ We want the scraper to return 'Yes' if the google home page has the title 'Google', so add a config value right above `unavailable_regexp` named `available_regexp` with the value `'<title>Google</title>'`.  Note that this is an [RE2 regular expression pattern](https://golang.org/pkg/regexp/syntax/).

_12._ We want the scraper to return 'No' if the google home page does NOT have the title 'Google', so replace the `unavailable_regexp` value with `'<title>[^<]+</title>'`.  Note that the `standard_regexp` type always looks for a Yes match before looking for a No match.  If it finds neither, it will return Possible.

_13._ We're done!  Save the .yaml file and execute the command `./covidwa-scrapers-go test acme_test_2` (assuming you named your scraper acme_test_2).  This will run  your scraper once, and skip all other scrapers.

_14._ For more advanced usage, such as passing in headers, or doing a POST with body, or even chaining together calls, see the .yaml for examples.

#### To test the scraper named "acme_test"
```shell
covidwa-scrapers-go test acme_test
```

#### To run all scrapers one time
```shell
covidwa-scrapers-go once
```

#### To run all scrapers continuously (production mode)
```shell
covidwa-scrapers-go
```

## How to create new scrapers

Simply add an entry to covidwa-scrapers.yaml, and point dump_dir to an existing
directory and you're good to go.  You probably want to use scraper type
"standard_regexp" in most cases to pattern match against the scraped content.

## How to create custom scrapers

Implement the Scraper (and ScraperFactory) interfaces.  See solv and kroger for examples.

## Also

* Run an individual scraper once by invoking ``covidwa-scrapers-go test <scraper_name>``
* Fill in from_email_address and smtp fields to enable email notifications for errors/changes
