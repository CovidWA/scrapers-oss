#!/usr/bin/env python

from datetime import date, timedelta
import json
import logging

from ScraperBase import ScraperBase
from Common import Status, SaveHtmlToTable, LIMITED_THRESHOLD
from util.BlockItNow import BlockItNowAPI, ONSITE_VACCINATION_PROCEDURE_ID

# See BlockItNowAPI for more details
BLOCKITNOW_PROFILE_IDS = {
    # "schedule_1": "7ebb13c7-3bd8-428a-8d22-9cc73ce0e72d",
    # "schedule_2": "cd599dfb-edfd-4b65-97ba-30391f585f14",
    # "schedule_3": "afb79236-f121-49df-b232-d628995b1356",
    # "schedule_4": "63060557-1f9a-439c-9b3d-da095e5a067d",
    # "schedule_5": "318dda1a-579d-4006-bdc4-94e750ba7888",
    "schedule_1": "d48f90a2-0ec5-40bf-89fd-24b8cea8994f",
    "schedule_2": "96d2da13-a51e-412e-91b7-80db300256d3",
    "schedule_3": "b93f10c9-c158-4c1a-ba62-a4760ea645b3",
    "schedule_4": "f0eb0f37-68e7-447b-98eb-3bd67b3cdc46",
    "schedule_5": "60ffd886-d094-4b7b-805a-8f8ea0aeb43a",
}
BLOCKITNOW_PROCEDURE_IDS = [
    ONSITE_VACCINATION_PROCEDURE_ID,
]

# days out to check for availability
# not sure if there's a limit here, keeping it small-ish to be safe
DAYS_OUT = 14


class PCHSGatewayCenter(ScraperBase):

    def __init__(self):
        self.LocationName = (
            "Peninsula Community Health Services - Gateway Center"
        )
        self.API = BlockItNowAPI()
        self.Keys = ["pchs_gateway_center"]

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

    scraper = PCHSGatewayCenter()
    keys, case, text = scraper.MakeGetRequest()

    logging.debug(f"keys={keys} case={case}")
