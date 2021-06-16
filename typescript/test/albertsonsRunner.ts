import {albertsonsHandler2 as handlerUnderTest} from '../src/handlers';

async function run() {
  await handlerUnderTest(undefined, undefined, () => true);
}

if (module === require.main) {
  run();
}
