#!/usr/bin/env python

from datetime import date, timedelta
import json
import logging

from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable, LIMITED_THRESHOLD
from util.BlockItNow import BlockItNowAPI, ONSITE_VACCINATION_PROCEDURE_ID

# See BlockItNowAPI for more details
BLOCKITNOW_PROFILE_IDS = {
    "schedule_1": "f5a84f8f-be4d-45c2-b558-67774a8896be",
    "schedule_2": "ccc7fff5-f3b6-490f-918e-6cda686c7de3",
    "schedule_3": "de26b1bf-b053-4b09-8ce8-f0b1aca9ce5d",
    "schedule_4": "f27f327a-09c2-49a9-b8df-727ed961089e",
    "schedule_5": "2d76bbdd-10e4-41e0-b560-2f95fee3f2bf",
}
BLOCKITNOW_PROCEDURE_IDS = [
    ONSITE_VACCINATION_PROCEDURE_ID,
]

# days out to check for availability
# not sure if there's a limit here, keeping it small-ish to be safe
DAYS_OUT = 14


class PCHSNorthMasonGym(ScraperBase):

    def __init__(self):
        self.LocationName = (
            "Peninsula Community Health Services - "
            "North Mason School District Campus Gymn"
        )
        self.API = BlockItNowAPI()
        self.Keys = ["pchs_north_mason_gym"]

    @SaveHtmlToTable
    def MakeGetRequest(self):
        START_DATE = date.today().strftime("%Y-%m-%d")
        END_DATE = (
            date.today() + timedelta(days=DAYS_OUT)
        ).strftime("%Y-%m-%d")

        n_available_slots, slots_per_schedule = (
            self.API.get_slots_per_schedule(
                start_date=START_DATE,
                end_date=END_DATE,
                profile_ids=BLOCKITNOW_PROFILE_IDS,
                procedure_ids=BLOCKITNOW_PROCEDURE_IDS,
            )
        )

        # Everything begins as possible
        case = Status.POSSIBLE
        if n_available_slots == 0:
            summary = slots_per_schedule
            logging.info(
                f"{ self.LocationName } no slots found, set status to waitlist"
            )
            case = Status.NO
        else:
            if n_available_slots > LIMITED_THRESHOLD:
                case = Status.YES
            else:
                case = Status.LIMITED
            try:
                summary = self.API.summarize_availability(slots_per_schedule)
            except Exception as e:
                summary = f"""
                Couldn't summarize, got error: { repr(e) }
                Check response payload for changes -
                likely still slots available, so status is YES
                """

            logging.info(
                f"""
                { self.LocationName } has available slots
                (checked between { START_DATE } and { END_DATE }):
                { summary }
                """
            )

        logging.info(f"Returning status {case}")

        return self.Keys, case, json.dumps(summary)


if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)

    scraper = PCHSNorthMasonGym()
    keys, case, text = scraper.MakeGetRequest()

    logging.debug(f"keys={keys} case={case}")
