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

// DEPRECATED, see golang scraper "providence_spokane_teachhc"

import {SignUpGeniusScraper} from './signUpGeniusBase';

export class SpokaneProvidenceTeachScraper extends SignUpGeniusScraper {
  constructor() {
    super('Spokane Providence Teaching Health Center', 'spokane_prov_teachhc');
    this.url = 'https://www.signupgenius.com/go/phase1a1bsthc';
  }
}
