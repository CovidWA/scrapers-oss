#!/usr/bin/env python

from datetime import datetime
import logging
from typing import Any, Dict, List

from python_graphql_client import GraphqlClient

"""
Simple utility class for interacting with BlockItNow GraphQL API

BlockItNow is a scheduling web app that some providers are using to
manage their scheduling

Slots are scheduled through app.blockitnow.com, which has a GraphQL API
IDs and GraphQL queries come from inspection of
health services provider site (EX: https://www.pchsweb.org/)
and its network requests to api.blockitnow.com

There are multiple schedules (representing a set of slots?) for a location
and each has its own profile ID

There's also speciality ID in response payload,
but the profile ID plus the procedure ID
should be enough for now
"""

# There seems to be just the one procedure ID used right now by the schedules,
# I think it represents "onsite vaccination"
# ONSITE_VACCINATION_PROCEDURE_ID = "08902fd3-2c13-4b66-a52b-ec5938c8b178"
ONSITE_VACCINATION_PROCEDURE_ID = "468129ce-1d13-4114-92aa-78e2a3b04da5"


class BlockItNowAPI(object):
    URL = "https://api.blockitnow.com"
    GraphQLClient = GraphqlClient(endpoint=URL)

    def check_for_slots(
        cls,
        start_date: str,
        end_date: str,
        profile_id: str,
        procedure_id: str
    ) -> List[Any]:
        query = """
        query GetConsumerSchedulingProfileSlotsQuery($procedureId: ID!, $profileId: ID!, $start: String, $end: String) {
          getConsumerSchedulingProfileSlots(procedureId: $procedureId, profileId: $profileId, start: $start, end: $end) {
            id
            start
            end
            status
          }
        }
        """
        variables = {
            "procedureId": procedure_id,
            "profileId": profile_id,
            "start": start_date,
            "end": end_date,
        }

        logging.debug(f"Checking slots for profile ID { profile_id }")
        data = cls.GraphQLClient.execute(query=query, variables=variables)
        logging.debug(f"Blockitnow GraphQL query response: { data }")

        slots = data.get("data", {}).get(
            "getConsumerSchedulingProfileSlots", []
        )

        return slots

    def summarize_availability(
        cls, slots_per_schedule: Dict[str, Any]
    ) -> Dict[str, Any]:
        # get min and max available times for each schedule
        summary = {}
        for schedule, slots in slots_per_schedule.items():
            # blockitnow returns slots as 3hr blocks, with status field
            # example slot payload:
            # {
            #   'end': '2021-03-04T14:21:00',
            #   'id': 'e8c0ad8a-6f53-5754-916a-a69cac4996b6',
            #   'start': '2021-03-04T14:18:00',
            #   'status': 'free'
            # }
            for slot in slots:
                start_ts = datetime.strptime(
                    slot["start"], "%Y-%m-%dT%H:%M:%S"
                )
                if schedule not in summary:
                    summary[schedule] = {
                        "first_available_at": start_ts,
                        "last_available_at": start_ts,
                    }
                else:
                    if start_ts < summary[schedule]["first_available_at"]:
                        summary[schedule]["first_available_at"] = start_ts
                    if start_ts > summary[schedule]["last_available_at"]:
                        summary[schedule]["last_available_at"] = start_ts

        # convert datetimes to str so JSON serializable
        for avail_dates in summary.values():
            avail_dates["first_available_at"] = (
                avail_dates["first_available_at"].strftime("%Y-%m-%d")
            )
            avail_dates["last_available_at"] = (
                avail_dates["last_available_at"].strftime("%Y-%m-%d")
            )

        return summary

    def get_slots_per_schedule(
        cls,
        start_date: str,
        end_date: str,
        # dict of schedule -> profile_id
        profile_ids: Dict[str, str],
        procedure_ids: List[str]
    ) -> (int, Dict[str, Any]):
        """
        Until we see differently (eg other provider sites), just align with how PCHS
        is using BlockItNow - eg, each profile ID is a specific "schedule"
        """
        n_available_slots = 0
        slots_per_schedule = {}

        for schedule, profile_id in profile_ids.items():
            for procedure_id in procedure_ids:
                slots = cls.check_for_slots(
                    start_date, end_date, profile_id, procedure_id
                )
                if slots:
                    slots_per_schedule.update({schedule: slots})
                    n_available_slots += len(slots)

        return n_available_slots, slots_per_schedule
