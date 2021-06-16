import {Browser, HTTPResponse, Page} from 'puppeteer-core';
import {Clinic} from '../helpers/getClinics';
import {error, log} from '../logger';
import {getPage} from '../puppeteer';
import {Scraper, ScrapeResult} from '../scraper';

class PeaceHealthScraper extends Scraper {
  private static URL =
    'https://myphschedule.peacehealth.org/Myphschedule/openscheduling';

  private locationName: string;

  constructor(humanName: string, locationName: string, key: string) {
    super(humanName, key);

    this.locationName = locationName;
  }

  async scrape(browser: Browser) {
    return this.scrapeWithRetry(browser, 5);
  }

  async scrapeWithRetry(browser: Browser, remainingRetries: number) {
    if (remainingRetries === 0) {
      error(`Unable to scrape ${this.locationName} location after 5 retries`);
      this.alarm = true;
      this.result = ScrapeResult.POSSIBLE;
      return;
    }
    log(
      `Attempting to scrape ${this.locationName} location, attempt #${
        5 - remainingRetries + 1
      }`
    );

    const page = await getPage(browser, 'Peace Health', PeaceHealthScraper.URL);
    await page.waitForTimeout(1000);
    try {
      await completeFormSection(
        page,
        '.specialtyList',
        'Covid Vaccination Clinic'
      );
      await page.waitForTimeout(1000);
      await completeFormSection(page, '.locationList', 'All locations');
      if (await hasZeroAvailability(page)) {
        this.result = ScrapeResult.NO;
      } else {
        await page.waitForTimeout(1000);
        await filterAppointmentsByLocation(page, this.locationName);
        await page.waitForTimeout(1000);

        const openings = await page.waitForSelector('.openingsData', {
          visible: true,
        });

        const appointmentSlots =
          (await openings?.$$('.card.withProvider')) ?? [];

        this.result =
          appointmentSlots.length > 0 ? ScrapeResult.YES : ScrapeResult.NO;
      }
    } catch (err) {
      error(err);
    } finally {
      await page.close();
    }

    if (this.result === ScrapeResult.UNKNOWN) {
      await this.scrapeWithRetry(browser, remainingRetries - 1);
    }
  }
}

async function completeFormSection(
  page: Page,
  formSectionClass: string,
  buttonText: string
) {
  const specialtyButtonContainer = await page.waitForSelector(
    formSectionClass,
    {visible: true, timeout: 3000}
  );

  const [button] =
    (await specialtyButtonContainer?.$x(
      `//li//span[contains(text(),"${buttonText}")]`
    )) ?? [];

  await button?.click();
}

async function hasZeroAvailability(page: Page) {
  await page.waitForTimeout(3000);
  const openings = await page.waitForSelector('.openingsData', {
    visible: true,
    timeout: 3000,
  });
  const openingsClassProperty = await openings?.getProperty('className');
  const openingsClasses: string =
    (await openingsClassProperty?.jsonValue()) ?? '';

  return openingsClasses.includes('openingsNoData');
}

async function filterAppointmentsByLocation(page: Page, locationName: string) {
  const locationControl = await page.waitForSelector('.locationControl a', {
    visible: true,
    timeout: 3000,
  });
  await locationControl?.click();
  await page.waitForTimeout(1000);

  const locationSelectionPane = await page.waitForSelector('#locationGroup', {
    visible: true,
    timeout: 3000,
  });

  const [
    uncheckAllButton,
    applyButton,
    [desiredLocationCheckbox] = [],
  ] = await Promise.all([
    locationSelectionPane?.$('.selectNone'),
    locationSelectionPane?.$('.applyFilter'),
    locationSelectionPane?.$x(
      `//div[contains(@class, "departmentDetails")][span[contains(text(),"${locationName.toUpperCase()}")]]/input`
    ),
  ]);

  await uncheckAllButton?.click();
  await desiredLocationCheckbox?.click();

  await Promise.all([
    page.waitForResponse(
      (response: HTTPResponse) =>
        response
          .url()
          .includes(
            'myphschedule.peacehealth.org/myphschedule/OpenScheduling/OpenScheduling/GetScheduleDays'
          ),
      {timeout: 3000}
    ),
    applyButton?.click(),
  ]);
}

export function getPeaceHealthScrapers(
  clinics: Array<Clinic>
): PeaceHealthScraper[] {
  return clinics
    .filter(({url}) => url?.includes('peacehealth.org'))
    .filter(({key}) => key?.startsWith('peace_health'))
    .filter(({name}) => name?.includes('-'))
    .map(({name, key}) => {
      const [, locationName] = name.split(/\s+-\s+/);
      return new PeaceHealthScraper(name, locationName, key);
    });
}
