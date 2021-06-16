// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import * as fs from 'fs';
import * as path from 'path';
import {log} from './logger';
import {Browser, HTTPResponse} from 'puppeteer-core';
// TODO add logic for this ./dump local and from lambda /tmp
let dumpFolder = './dump';

if (process.env.AWS_LAMBDA) {
  dumpFolder = '/tmp';
}

// define proper type for a shared browser object: some TypeScript magic
type ThenArg<T> = T extends PromiseLike<infer U> ? U : T;

export async function getPage(
  browser: Browser,
  key: string,
  url: string,
  respCallback?: (response: HTTPResponse) => void
) {
  const page = await browser.newPage();
  if (respCallback) {
    page.on('response', respCallback!);
  }
  await page.goto(url);
  const content = await page.content();

  // Dump the contents of the page for future analysis if needed
  if (!fs.existsSync(dumpFolder)) {
    await fs.promises.mkdir(dumpFolder);
  }
  const now = new Date().getTime();
  const filename = path.join(dumpFolder, `${key}-${now}`);
  await fs.promises.writeFile(filename, content);
  log(`Saved HTML page: ${filename}`);

  return page;
}

export type Page = ThenArg<ReturnType<typeof getPage>>;
type ArrayElement<
  ArrayType extends readonly unknown[]
> = ArrayType extends readonly (infer ElementType)[] ? ElementType : never;
export type Frame = ArrayElement<ReturnType<Page['frames']>>;

export async function isVisible(
  page: Page | Frame,
  selector: string,
  noThrow = false
) {
  const elementHandle = await page.$(selector);
  if (!elementHandle) {
    if (noThrow) {
      return false;
    }
    throw new Error(`Element ${selector} was not found`);
  }
  const bbox = await elementHandle.boundingBox();
  return bbox !== null;
}

export async function wait(
  page: Page,
  selector: string,
  timeout = 5000,
  noThrow = false,
  visibility?: boolean
) {
  try {
    if (visibility !== undefined) {
      await page.waitForSelector(selector, {
        visible: visibility,
        timeout: timeout,
      });
    } else {
      await page.waitForSelector(selector, {
        timeout: timeout,
      });
    }
    return true;
  } catch (e) {
    if (noThrow && e.constructor.name === 'TimeoutError') {
      return false;
    } else {
      throw e;
    }
  }
}

export async function innerHTML(page: Page, selector: string) {
  const elementHandle = await page.$(selector);
  if (!elementHandle) {
    throw new Error(`Element ${selector} was not found`);
  }
  const innerHTML = elementHandle.evaluate(
    node => (node as HTMLElement).innerHTML
  );
  return innerHTML;
}

export async function waitAndClick(
  page: Page,
  selector: string,
  waitTimeout = 10000,
  extraTimeout = 500,
  retries = 0,
  retryTimeout = 1000
) {
  await page.waitForSelector(selector, {
    visible: true,
    timeout: waitTimeout,
  });
  await sleep(extraTimeout);

  await click(page, selector, retries, retryTimeout);
}

export async function click(
  page: Page,
  selector: string,
  retries = 10,
  retryTimeout = 1000
) {
  let clicked = false;
  let retryCount = 0;
  while (!clicked) {
    try {
      await page.click(selector);
    } catch (e) {
      retryCount++;
      if (retryCount > retries) {
        throw e;
      }
      await sleep(retryTimeout);
      continue;
    }
    clicked = true;
    log(`Click on '${selector}' succeeded after ${retryCount} retries`);
  }
}

export function sleep(timeout: number) {
  return new Promise(resolve => setTimeout(resolve, timeout));
}
