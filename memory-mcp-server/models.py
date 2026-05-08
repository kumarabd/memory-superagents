from enum import Enum
from pydantic import BaseModel


class MemoryType(str, Enum):
    preference = "preference"
    profile_fact = "profile_fact"
    project_context = "project_context"
    decision = "decision"
    task = "task"
    event = "event"
    problem = "problem"
    solution = "solution"
    learning = "learning"
    question = "question"
    plan = "plan"
    constraint = "constraint"
    credential_reference = "credential_reference"
    relationship = "relationship"
    routine = "routine"
    artifact = "artifact"
    conversation_summary = "conversation_summary"
    correction = "correction"
    feedback = "feedback"
    observation = "observation"
    hypothesis = "hypothesis"
    experiment = "experiment"
    capability = "capability"
    policy = "policy"
    identity = "identity"


class SearchFilters(BaseModel):
    memory_type: MemoryType | None = None
    scope: str | None = None
    project: str | None = None
