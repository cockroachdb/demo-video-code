import os
import psycopg
import uuid
from langchain_openai import ChatOpenAI
from langchain_core.messages import HumanMessage
from langchain_core.runnables.history import RunnableWithMessageHistory
from langchain_core.runnables import Runnable
from langchain_postgres import PostgresChatMessageHistory
from langchain_core.chat_history import BaseChatMessageHistory
from typing import List
from langchain_core.messages import BaseMessage


DATABASE_URL = os.environ["DATABASE_URL"]

conn = psycopg.connect(DATABASE_URL)


class LimitedPostgresChatMessageHistory(BaseChatMessageHistory):
    def __init__(self, table_name: str, session_id: str, sync_connection, limit: int = 3):
        self.history = PostgresChatMessageHistory(
            table_name, session_id, sync_connection=sync_connection
        )
        self.limit = limit

    @property
    def messages(self) -> List[BaseMessage]:
        all_messages = self.history.messages
        return all_messages[-self.limit :] if len(all_messages) > self.limit else all_messages

    def add_message(self, message: BaseMessage) -> None:
        self.history.add_message(message)

    def clear(self) -> None:
        self.history.clear()


def get_history(session_id: str) -> BaseChatMessageHistory:
    return LimitedPostgresChatMessageHistory(
        "chat_history", session_id, sync_connection=conn, limit=3
    )


llm = ChatOpenAI(model="gpt-4o-mini")

chat_with_history = RunnableWithMessageHistory(
    llm,
    get_history,
)

session_id = str(uuid.uuid4())
while True:
    user_input = input("[ME ->] ")

    if user_input.strip().lower() in ["exit", "quit"]:
        break

    if not user_input.strip():
        continue

    out = chat_with_history.invoke(
        [HumanMessage(content=user_input)],
        config={"configurable": {"session_id": session_id}},
    )

    print(f"[<- AI] {out.content}\n")
