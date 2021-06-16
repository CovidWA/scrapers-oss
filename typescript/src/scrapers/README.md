# Getting Started

Read the `example.ts` for a general framework. All the scrapers should extend the `Scraper` class from the above directory.
The method they implement is `scrape()`, but it's good practice to modularize into helper functions like `checkVaccinationStatus()`.

Each site needs a slightly different approach. You may need to make your scraper click through a form, like in `seattlechildren.ts`.

# Puppeteer

Puppeteer is a web scraping library for Node.
There are some helper functions like `getPage()` and `waitAndClick()` included in the `puppetteer.ts` file from the above directory.