import uuid
from langchain_openai import ChatOpenAI
from langchain_core.messages import HumanMessage


llm = ChatOpenAI(model="gpt-4o-mini")

session_id = str(uuid.uuid4())
while True:
    user_input = input("[ME ->] ")

    if not user_input.strip():
        continue

    out = llm.invoke(
        [HumanMessage(content=user_input)],
        config={"configurable": {"session_id": session_id}},
    )

    print(f"[<- AI] {out.content}\n")
