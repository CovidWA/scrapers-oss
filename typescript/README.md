# Initial setup of TypeScript project

## Config (Dev vs. Prod)

There must be a `config.json` file in this directory before running `npm run compile`.
Any time you change the config file, this directory must be recompiled.

A sample config file for the Test environment is included.  Note that the test backend (`covidwa-backend-test.herokuapp.com`) uses the sites-dev table.

If you need to test a scraper that uses the recaptcha bypass service, ask for the key on Slack. 

## Running the TypeScript scrapers

Get Node.js v14 from the official site, or use
[`nvm`](https://github.com/nvm-sh/nvm#install--update-script).

Then run:

```sh
$ cd typescript
$ cp config.json.sample config.json # sample config file w/ Test environment configuration
$ npm install
$ npm run compile   # to compile TypeScript code
$ npm start         # to run all scrapers locally
```

## Running Puppeteer Headless
 Locally puppeteer runs headfull for debugging. If you would like to run headless, set an environment variable:

 ```sh
 export AWS_LAMBDA_FUNCTION_NAME=typescript
 ```
see https://github.com/alixaxel/chrome-aws-lambda/wiki/HOWTO:-Local-Development for more information.

## Testing individual scrapers

To run scrapers individually, first build:
```sh
$ cd typescript
$ cp config.json.sample config.json # sample config file w/ Test environment configuration
$ npm install
```

Then run the test script out of the build directory (testing the riteAid scraper in this example):
```sh
$ node ./build/test/index.js riteAid
```
The command above runs the scraper in headed mode, with a fixed resolution, console and slowMo enabled.

To run the scraper with a full size browser window in normal speed:
```sh
$ node ./build/test/index.js <scraper_name> defaults
```

Finally, to run the scraper in headless mode:
```sh
$ node ./build/test/index.js <scraper_name> headless
```

## Possible errors

Invalid protocol error should have been fixed by a recent PR that matched the request type to
the URL.
