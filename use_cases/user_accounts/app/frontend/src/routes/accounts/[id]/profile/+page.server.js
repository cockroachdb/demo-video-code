export const load = async ({params}) => {
  const response = await fetch(`http://localhost:3000/accounts/${params.id}/profile`, {
    method: "GET",
    headers: {
      "Content-Type": "application/json"
    }
  })
  return await response.json();
}