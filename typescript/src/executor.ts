import {Browser} from 'puppeteer-core';
import {Scraper, ScrapeResult} from './scraper';
import {color, error, log} from './logger';
import * as chromium from 'chrome-aws-lambda';
import * as rimraf from 'rimraf';

export type ScraperExecutor = (
  event: undefined,
  context: undefined,
  callback: Function
) => void;

// https://stackoverflow.com/a/12646864/2231691
export function shuffleArray<T>(array: T[]) {
  for (let i = array.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [array[i], array[j]] = [array[j], array[i]];
  }
}

const getBrowser = async (): Promise<Browser> => {
  return await chromium.puppeteer.launch({
    args: chromium.args,
    defaultViewport: chromium.defaultViewport,
    executablePath: await chromium.executablePath,
    headless: chromium.headless,
    ignoreHTTPSErrors: true,
  });
};

export const ScraperHandler = (...scrapers: Scraper[]): ScraperExecutor => {
  return async (event, context, callback) => {
    const result = null;
    // start clean
    if (process.env.AWS_LAMBDA) {
      rimraf.sync('/tmp/*');
      log('Finished removing files from /tmp directory');
    }

    let browser = undefined;
    for (const scraper of scrapers) {
      if (scraper.needsBrowser()) {
        browser = await getBrowser();
        break;
      }
    }

    shuffleArray(scrapers);

    console.log(`Total scrapers initialized: ${scrapers.length}`);
    console.log(
      scrapers.map(scraper => `${scraper.getHumanName()} ${scraper.getKey()}`)
    );
    let counter = 0;

    for (const scraper of scrapers) {
      ++counter;
      let logPrefix = '';
      try {
        logPrefix = `${counter}/${scrapers.length} ${scraper.getHumanName()}:`;
      } catch (err) {
        error(`Error initializing scraper #${counter}: ${err}`);
        logPrefix = `${counter}/${scrapers.length}:`;
      }
      try {
        await scraper.scrape(browser);

        log(`${logPrefix} ${color(scraper.getResult())}`);
        if (scraper.getResult() === ScrapeResult.POSSIBLE) {
          log(`${logPrefix} Got Possible, retrying once more...`);

          await scraper.scrape(browser);
          log(`${logPrefix} ${color(scraper.getResult())}`);
        }
      } catch (err) {
        error(`${logPrefix} Error scraping: ${err}`);
      }
      try {
        const response = await scraper.report();
        log(`${logPrefix} reported. Backend response: ${response}`);
      } catch (err) {
        error(`${logPrefix} Error reporting result: ${err}`);
      }
    }
    log('Scraping session completed.');

    if (browser !== undefined) {
      try {
        await browser.close();
      } catch (err) {
        log(`[ERROR]: could not close browser.\n${err}`);
      }
    }

    return callback(null, result);
  };
};
