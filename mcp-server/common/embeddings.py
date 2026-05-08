import os

from openai import AsyncOpenAI

_client: AsyncOpenAI | None = None


def _get_client() -> AsyncOpenAI:
    global _client
    if _client is None:
        api_key = os.environ.get("OPENAI_API_KEY")
        if not api_key:
            raise RuntimeError("OPENAI_API_KEY environment variable is required")
        _client = AsyncOpenAI(api_key=api_key, timeout=30.0)
    return _client


async def close() -> None:
    global _client
    if _client is not None:
        await _client.close()
        _client = None


async def embed(text: str) -> list[float]:
    response = await _get_client().embeddings.create(
        model="text-embedding-ada-002",
        input=text,
    )
    return response.data[0].embedding
