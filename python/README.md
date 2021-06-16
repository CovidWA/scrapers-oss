# COVID Washington Vaccine Finder with Python

To make a python scraper you are in the right place.

## Install instructions with venv

Create new virtual environment in directory (python must be on PATH, i am using
3.9.1 but 3.7+ will work)

So to activate and then install the python requirements

```shell
# Get the secret from Slack
export API_SECRETWA='XXXX' #secret for Test environment
export API_HOSTWA='covidwa-backend-test.herokuapp.com' #host for Test environment
python3 -m venv .env
source .env/bin/activate
pip install -r requirements.txt
```

### Installation with conda on MacOS

This works with MacOS Big Sur and assumes you have [homebrew](https://brew.sh) installed

```shell
# if you do not have homebrew
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
brew install bash make python miniconda
```

Note if you install python this way, then you will need to update your
.bash_profile

```shell
[[ $PATH =~ /usr/local/opt/python/libexec/bin ]] || export PATH="/usr/local/opt/python/libexec/bin:$PATH"
```

And now you can run the conda environment assuming we call it covidwa

```shell
make install
conda activate covidwa
export API_SECRETWA='XXXXXXX' #secret for Test environment
export API_HOSTWA='covidwa-backend-test.herokuapp.com' #host for Test environment
# now you can run locall
...
# when you done you just run
conda deactivate
``````

## Create new scraper

Each scraper is a new class based on the main Scraper class.

Go to Template.py is the raw file

You can use the following as templates:

1. Go to [Airtable
   Production](https://airtable.com/tblSVU8xMVpoJBBsP/viw77Vi1N0xO7Xsbz?blocks=hide)
   and get a scraper that is not yet done or search for you as an owner. Add a
   unique key in the `key` section. This really has to be unique and can be any
   string so choose wisely so something like `evergreen_hospital` or you could
   use a GUID if you want but the convention is
   [snake_case](https://en.wikipedia.org/wiki/Letter_case#Special_case_styles)
   and lower case
2. Then use Chrome
   Inspector to look at the site to see what elements you want to scrape.
3. If you aren't familier, learn [Beautiful
   Soup](https://www.tutorialspoint.com/beautiful_soup/index.htm)
4. If the site has multiple slots that need to be filled look at [Evergreen
   State.py](EvergreenState.py). This looks through items and then if there
   are any entries that do not say "Already Filled" it will mark it as
   possible.

## Notes on making a new scraper

To make a new scraper set some variables:

1. set URL - This is the URL where you get the data
2. set Keys - must match entry in airtable under the `key` attribute
_WARNING_ Keys must be an array, this is because multiple entries in Airtable are
updated by one scraper, if you enter multiple keys, all of these respective
airtable entries will be updated with the result of the scraper on every run
3. set LocationName - This is used as a prefix and is set to the text name like
"Evergreen Internists" as an example.
4. Find FailureCase - This involves going to the website and finding a string that
represents vaccines not being available e.g "Auburn is not currently allocating
any new vaccine appointments"
5. Find which type of element contains your failure case and update
soup.find_all('p') to use that element that is all paragraphs. Some common
elements to look for are div, span, p, li, etc etc
6. Run file by instantiating the class and check the failure case is met. On
   Linux/Mac systems, you can insert `#!/usr/bin/env python` at the top and add
   a check for __name__ is "__main__" and run it like a script.
7. Test your scraper against the Test backend by making sure
   `export API_SECRETWA` and `API_HOSTWA` are set. Start with local checks.  You may also test against the production backend by unsetting `API_HOSTWA` and setting the appropriate value for `API_SECRETWA`.
8. Import new scraper into ScrapeAllAndSend.py Add new scraper into
lstActiveScrapers (as an object) Submit PR for your change and wait for
deployment (or ping @kxdan)

```shell
[[ $PATH =~ /usr/local/opt/python/libexec/bin ]] || export PATH="/usr/local/opt/python/libexec/bin:$PATH"
```

And now you can run the conda environment assuming we call it covidwa

```shell
make install
conda activate covidwa
export API_SECRETWA='XXXXXX' #secret for Test environment
export API_HOSTWA='covidwa-backend-test.herokuapp.com' #host for Test environment
# now you can run locall
...
# when you done you just run
conda deactivate
``````

You can use the following as templates:

1. Go to [Airtable
   Production](https://airtable.com/tblSVU8xMVpoJBBsP/viw77Vi1N0xO7Xsbz?blocks=hide)
   and get a scraper that is not yet done or search for you as an owner. Add a
   unique key in the `key` section. This really has to be unique and can be any
   string so choose wisely so something like `evergreen_hospital` or you could
   use a GUID if you want but the convention is
   [snake_case](https://en.wikipedia.org/wiki/Letter_case#Special_case_styles)
   and lower case
2. Then use Chrome
   Inspector to look at the site to see what elements you want to scrape.
3. If you aren't familier, learn [Beautiful
   Soup](https://www.tutorialspoint.com/beautiful_soup/index.htm)
4. If the site has multiple slots that need to be filled look at [Evergreen
   State.py](EvergreenState.py). This looks through items and then if there
   are any entries that do not say "Already Filled" it will mark it as
   possible.
5. If you have a simple site with just one line of text that says available
   then look at [EvergreenIntern.py](EvergreenIntern.py)

## Running and testing the scraper

To test the scraper, you should instantiate the class and get going. See the
bottom of EvergreenState.py for the sample code.

To run an individual scraper, first run a build

```shell
./bin/build.sh
```

Then execute the scraper script in the lambda-stage directory
(in this example, acuity)
```shell
python3 ./lambda-stage/AcuityScheduling.py
```

## Getting your scraper integrated into production

When you are done with your scraper, edit
[ScrapeAllAndSend.py](ScrapeAllAndSend.py) to add your new class.

Then issue a PR and this will get picked up by the backend automatically.
