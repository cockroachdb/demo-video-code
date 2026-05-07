import os
import psycopg
import uuid
from langchain_openai import ChatOpenAI
from langchain_core.messages import HumanMessage
from langchain_core.runnables.history import RunnableWithMessageHistory
from langchain_postgres import PostgresChatMessageHistory


DATABASE_URL = os.environ["DATABASE_URL"]

conn = psycopg.connect(DATABASE_URL)


def get_history(session_id: str) -> PostgresChatMessageHistory:
    return PostgresChatMessageHistory("chat_history", session_id, sync_connection=conn)


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
