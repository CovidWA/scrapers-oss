import {Browser, Protocol, Page, HTTPResponse} from 'puppeteer-core';
import RecaptchaPlugin, {
  PuppeteerExtraPluginRecaptcha,
} from 'puppeteer-extra-plugin-recaptcha';

import * as config from '../../config.json';
import {error, log} from '../logger';
import {getPage} from '../puppeteer';

export class ReCAPTCHAServiceError extends Error {}

export interface CredentialManager {
  useCredentials<T>(
    browser: Browser,
    credentialConsumingFn: CredentialConsumingFunction<T>,
    credentialUseExtras?: CredentialUseExtras
  ): Promise<T>;
}

export type CredentialConsumingFunction<T> = (
  credentials: ApiRequestCredentials
) => Promise<T>;

export interface ApiRequestCredentials {
  csrfKey: string;
  cookies: Protocol.Network.CookieParam[];
}

export interface CredentialUseExtras {
  doesRequestDegradeCredentials: boolean;
}

type InitializationOptions = {
  initialCredentials?: ApiRequestCredentials;
  initialCredentialCount?: number;
  overrideUrl?: string;
  regionalRefreshId?: string;
  canIgnoreAgeVerification: boolean;
};

const MAX_CSRF_KEY_REQUESTS = 26;

const REQUIRED_COOKIE_NAMES = new Set([
  'JSESSIONID',
  'AWSALBCORS',
  'AWSALB',
  'AWSALBTGCORS',
  'AWSALBTG',
]);

export class AlbertsonsCredentialManager implements CredentialManager {
  private currentCredentials?: ApiRequestCredentials;
  private credentialUseCount: number;
  private refreshOverrideUrl?: string;
  private regionalRefreshId: string;
  private recaptcha: PuppeteerExtraPluginRecaptcha;
  private canIgnoreAgeVerification: boolean;

  static forSearchClient(
    regionalRefreshId: string
  ): AlbertsonsCredentialManager {
    return new this({regionalRefreshId, canIgnoreAgeVerification: false});
  }

  static forRegisteredCompany(registeredCompanyInitialization: {
    initialCredentials?: ApiRequestCredentials;
    initialCredentialCount?: number;
    overrideUrl?: string;
  }): AlbertsonsCredentialManager {
    return new this({
      ...registeredCompanyInitialization,
      canIgnoreAgeVerification: true,
    });
  }

  private constructor(initializationOptions: InitializationOptions) {
    this.recaptcha = RecaptchaPlugin({
      provider: {
        id: '2captcha',
        // Needs a (paid :( ) 2captcha account
        token: config.recaptchaBypassServiceToken,
      },
      visualFeedback: true,
    });

    this.currentCredentials = initializationOptions?.initialCredentials;
    this.credentialUseCount =
      initializationOptions?.initialCredentialCount ?? 0;
    this.refreshOverrideUrl = initializationOptions?.overrideUrl;
    this.regionalRefreshId =
      initializationOptions?.regionalRefreshId ?? '1600115131031';
    this.canIgnoreAgeVerification =
      initializationOptions.canIgnoreAgeVerification;
  }

  get currentCredentialInfo() {
    return {
      credentials: this.currentCredentials,
      useCount: this.credentialUseCount,
    };
  }

  async useCredentials<T>(
    browser: Browser,
    credentialConsumingFn: CredentialConsumingFunction<T>,
    {doesRequestDegradeCredentials}: CredentialUseExtras = {
      doesRequestDegradeCredentials: true,
    }
  ): Promise<T> {
    if (
      this.credentialUseCount === MAX_CSRF_KEY_REQUESTS ||
      !this.currentCredentials
    ) {
      try {
        await this.refreshCredentials(browser);
      } catch (err) {
        throw new ReCAPTCHAServiceError(err.message);
      }
    }

    const result = await credentialConsumingFn(this.currentCredentials!).catch(
      async err => {
        // With the introduction of cached credentials for certain requests,
        // There's a chance those creds passed to us are stale/spent.
        // Try a refresh before completely erroring out.
        error(err);
        log(
          'Encountered an issue with the current credentials, might be due to a stale cache. Refreshing...'
        );
        try {
          await this.refreshCredentials(browser);
        } catch (err) {
          throw new ReCAPTCHAServiceError(err.message);
        }
        return credentialConsumingFn(this.currentCredentials!);
      }
    );
    if (doesRequestDegradeCredentials) {
      this.credentialUseCount++;
    }

    return result;
  }

  private async refreshCredentials(browser: Browser) {
    let page;
    try {
      log('Beginning credential refresh...');
      page = await getPage(
        browser,
        'AlbertsonsCredentialManager',
        this.refreshOverrideUrl ??
          `https://kordinator.mhealthcoach.net/vcl/${this.regionalRefreshId}`
      );
      this.currentCredentials = await this.captureApiRequestCredentials(
        page,
        this.recaptcha
      );
    } finally {
      if (page) {
        await page.close();
      }
    }

    log('Credential refresh successful');
    log(`Using csrfKey ${this.currentCredentials.csrfKey}`);
    this.currentCredentials.cookies.forEach(({name, value}) => {
      log(`Using cookie ${name}=${value}`);
    });
    this.credentialUseCount = 0;
  }

  private async captureApiRequestCredentials(
    page: Page,
    recaptcha: PuppeteerExtraPluginRecaptcha
  ): Promise<ApiRequestCredentials> {
    const verificationButtons = await page
      .waitForSelector('#covid_vaccine_search_questions_content input', {
        visible: true,
        timeout: 5000,
      })
      .catch(err => {
        if (this.canIgnoreAgeVerification) {
          log(
            "Didn't find age verification checkbox but it can be safely ignored for this site."
          );
          return null;
        }
        throw err;
      })
      .then(() => page.$$('#covid_vaccine_search_questions_content input'));

    await Promise.all(verificationButtons?.map(button => button.click()));

    await page.click('#covid_vaccine_search_questions_submit .btn-primary');

    const pharmacistLoginResponse = await Promise.race([
      this.vanquishRecaptcha(page, recaptcha),
      // Other times the API request is kicked off as soon as the submit button is clicked
      page.waitForResponse(
        (response: HTTPResponse) =>
          response.url().includes('loginPharmacistFromEmail.do'),
        {timeout: 240000}
      ),
    ]);

    const {csrfKey} = await pharmacistLoginResponse.json();
    const cookies = await page.cookies();

    return {
      csrfKey,
      cookies: cookies.filter(({name}) => REQUIRED_COOKIE_NAMES.has(name)),
    };
  }

  private async vanquishRecaptcha(
    page: Page,
    recaptcha: PuppeteerExtraPluginRecaptcha
  ) {
    // Sometimes recaptcha pops up as a result of clicking on the submit button and we need to solve it
    const recaptchaPrompt = await page.waitForXPath(
      '//*[@id="recaptchaRow" and not(contains(@style, "display: none"))]',
      {visible: true}
    );
    if (!recaptchaPrompt) {
      return page.waitForResponse((response: HTTPResponse) =>
        response.url().includes('loginPharmacistFromEmail.do')
      );
    }
    log('Recaptcha exists, attempting to solve');

    const [submitButton, {error}] = await Promise.all([
      page.waitForSelector(
        '#covid_vaccine_search_questions_submit .btn-primary'
      ),
      // This averages 30s and costs $2.99 per 1000 solves
      recaptcha.solveRecaptchas(page),
    ]);
    if (error) {
      throw Error(`Recaptcha error: ${error}`);
    }

    // My guess is because of how wonky the "suddenly appearing recaptcha" makes the UI,
    // puppeteer is having a hard time clicking on the submit button after recaptcha is solved without this
    const [clickAwayCoordinates, v2Challenge] = await Promise.all([
      submitButton!.boundingBox().then(coordinates => {
        const {x, y, height} = coordinates!;
        return {x: x - 15, y: y + height / 2};
      }),
      page
        .waitForXPath('//iframe[@title="recaptcha challenge"]', {
          visible: true,
          timeout: 3000,
        })
        .catch(() => null),
    ]);

    await page.mouse.click(clickAwayCoordinates.x, clickAwayCoordinates.y);
    // I don't know why they bother with the v2 challenge if we can just dismiss it by clicking outside its bounding box.
    if (v2Challenge) {
      log(
        'A surprise v2 challenge has appeared! CovidWA used "click outside it" - it was super effective!'
      );
      // Wait for half a second in this case so the UI can settle down
      await page.waitForTimeout(500);
    }

    await submitButton?.click();
    return page.waitForResponse((response: HTTPResponse) =>
      response.url().includes('loginPharmacistFromEmail.do')
    );
  }
}
