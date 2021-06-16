import path = require('path');

import puppeteer from 'puppeteer-extra';
import {Browser} from 'puppeteer-core';

import {Scraper} from '../src/scraper';
import {color, log} from '../src/logger';
import {Clinic, getClinics} from '../src/helpers/getClinics';

async function run(scraperName: string, ...moreArgs: string[]) {
  const builtScrapersDir = path.normalize(
    path.join(__dirname, '..', 'src', 'scrapers')
  );

  const scraperModule = require(path.join(builtScrapersDir, scraperName));
  const scraperFamilyFnName = Object.keys(scraperModule).find(name =>
    name.startsWith('get')
  );

  const scrapers = [];
  if (scraperFamilyFnName) {
    const clinics = await getClinics();
    const scraperFamilyFn: (clinics: Clinic[]) => Scraper[] =
      scraperModule[scraperFamilyFnName];
    scrapers.push(...scraperFamilyFn(clinics));
  } else {
    const ScraperClass = scraperModule[Object.keys(scraperModule)[0]];
    scrapers.push(new ScraperClass());
  }

  let puppeteerOpts: Object = {
    // The following lines are meant to work around a type clash puppeteer caused a few releases back:
    // https://github.com/berstend/puppeteer-extra/issues/428#issuecomment-778679665
    // eslint-disable-next-line @typescript-eslint/ban-ts-comment
    // @ts-ignore
    headless: false,
    slowMo: 150,
    devtools: true,
    args: [
      '--disable-setuid-sandbox',
      '--no-sandbox',
      '--window-position=000,000',
      '--window-size=1366,768',
    ],
  };

  if (moreArgs.length > 0 && moreArgs[0] === 'headless') {
    puppeteerOpts = {};
  } else if (moreArgs.length > 0 && moreArgs[0] === 'defaults') {
    puppeteerOpts = {
      headless: false,
    };
  }
  const browser: Browser = await puppeteer.launch(puppeteerOpts);
  console.log(`Running ${scrapers.length} scrapers`);

  for (const scraper of scrapers) {
    await scraper.scrape(browser);
    log(`${scraper.getHumanName()}: ${color(scraper.getResult())}`);
  }

  await browser.close();
}

if (module === require.main) {
  if (process.argv.length < 3) {
    log(
      `Usage: ${process.argv[0]} ${process.argv[1]} <scraper_name> [headless]|[defaults]`
    );
  } else {
    run(process.argv[2], ...process.argv.slice(3));
  }
}
