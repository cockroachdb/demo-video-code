export const load = async ({params}) => {
  const response = await fetch(`https://crdb-user-accounts.fly.dev/accounts/${params.id}/profile`, {
    method: "GET",
    headers: {
      "Content-Type": "application/json"
    }
  })
  return await response.json();
}