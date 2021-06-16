import {Scraper, ScrapeResult} from '../scraper';
import {Clinic, getClinicsByUrlKeyword} from '../helpers/getClinics';
import {log, error} from '../logger';
import {Page, getPage} from '../puppeteer';
import {Browser, HTTPResponse} from 'puppeteer-core';

// Generic Calendly Url Scraper
class CalendlyScraper extends Scraper {
  scrapeUrl: string;
  apptCount: number;

  constructor(humanName: string, key: string, scrapeUrl: string) {
    super(humanName, key);
    this.scrapeUrl = scrapeUrl;
    this.apptCount = 0;
  }

  async scrape(browser: Browser) {
    const page = await getPage(
      browser,
      this.key,
      this.scrapeUrl,
      this.apiInterceptor.bind(this)
    );

    const links = await page.$$('a[data-id="event-type"]');

    if (links.length > 0) {
      log(`${this.key}: Found ${links.length} event links`);

      for (const link of links) {
        await page.bringToFront();
        //copypasta from https://pocketadmin.tech/en/puppeteer-open-link-in-new-tab/
        const newPagePromise = new Promise<Page>(x =>
          browser.once('targetcreated', target => x(target.page()))
        );
        link.click({button: 'middle'});
        const page2 = await newPagePromise;
        page2.on('response', this.apiInterceptor.bind(this));
        await page2.bringToFront();
        await page2.waitForNavigation();

        await this.checkVaccinationStatus(page2);
        if (this.result === ScrapeResult.YES) {
          break;
        }

        await page2.close();
      }
    } else {
      log(`${this.key}: Found no event links, scraping url directly`);

      await this.checkVaccinationStatus(page);
    }

    log(`${this.key}: Total appts: ${this.apptCount}`);

    // Close the Chromium page
    await page.close();
  }

  private async checkVaccinationStatus(page: Page) {
    try {
      log(`${this.key}: Checking ${page.url()}`);

      let hasNextMonthButton = false;
      let monthsExamined = 0;
      do {
        await page.waitForSelector('.calendar-loader', {
          hidden: true,
          timeout: 10000,
        });

        const noDatesPopup = await page.$('.calendar-no-dates-button');
        if (!noDatesPopup) {
          const availableAppointmentTimes = await page.$$eval(
            'tbody.calendar-table tr td button',
            dateSelectors =>
              dateSelectors
                .filter(e => !e.hasAttribute('disabled'))
                .map(e => e.getAttribute('aria-label'))
          );

          if (availableAppointmentTimes.length > 0) {
            if (this.apptCount > this.limitedThreshold) {
              this.result = ScrapeResult.YES;
              return;
            }

            this.result = ScrapeResult.LIMITED;
          }
        }

        hasNextMonthButton = false;
        const nextMonthButton = await page.$(
          'button:not([disabled])[aria-label="Go to next month"]'
        );
        if (nextMonthButton) {
          hasNextMonthButton = true;
          monthsExamined++;
          await nextMonthButton.click();
        }
      } while (hasNextMonthButton && monthsExamined <= 6);

      if (this.apptCount <= 0) {
        this.result = ScrapeResult.NO;
      }

      return;
    } catch (err) {
      error(`Error scraping ${this.humanName}: ${err.toString()}`);
    } finally {
      // https://2ality.com/2013/03/try-finally.html - will happen even though there's a return in the try
      this.output = await page.content();
    }

    // Normally, if we reached here, we don't know what's going on
    this.result = ScrapeResult.POSSIBLE;
    this.alarm = true;
  }

  private async apiInterceptor(response: HTTPResponse): Promise<void> {
    if (response.url().includes('calendar/range')) {
      const slotsResp = await response.json();

      for (const day of slotsResp.days) {
        for (const spot of day.spots) {
          if (spot.status === 'available') {
            this.apptCount += spot.invitees_remaining;
          }
        }
      }
    }
  }
}

export function getCalendlyScrapers(clinics: Clinic[]): CalendlyScraper[] {
  const calendlyClinics = getClinicsByUrlKeyword('calendly', clinics);

  const calendlyScrapers = [];
  for (const clinic of calendlyClinics) {
    log(`Creating calendly scraper for ${clinic.key}`);

    calendlyScrapers.push(
      new CalendlyScraper(clinic.humanName, clinic.key, clinic.url)
    );
  }
  return calendlyScrapers;
}
