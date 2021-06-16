import {Scraper, ScrapeResult} from '../scraper';
import {getPage, Page} from '../puppeteer';
import {Browser} from 'puppeteer-core';

export class CleElumCentennial extends Scraper {
  private readonly url = 'https://www.co.kittitas.wa.us/health/default.aspx';

  private static NO_APPOINTMENTS_CONTENT =
    'APPOINTMENTS UNAVAILABLE AT THIS TIME';

  constructor() {
    super('CleElumCentennialScraper', 'CE17BFD3-9D16-42C1-92A8-706BF3162320');
  }

  async scrape(browser: Browser) {
    const page = await getPage(browser, this.key, this.url);
    this.output = await page.content();
    await this.getVaccinationStatus(page);
    return page;
  }

  getVaccinationStatus = async (page: Page) => {
    this.result = ScrapeResult.POSSIBLE;

    // currently there is no active appointment scheduler linked
    // TODO: once scheduler is online, update to scrape for v2 api results
    this.alarm = true;

    const centerDivSelector = 'div[class="notification-box danger1 center"]';
    const centerDivContent = await page.$eval(
      centerDivSelector,
      el => el.textContent
    );
    if (
      centerDivContent &&
      centerDivContent.indexOf(CleElumCentennial.NO_APPOINTMENTS_CONTENT) > -1
    ) {
      this.result = ScrapeResult.NO;
      this.alarm = false;
    }
  };
}
