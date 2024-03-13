import { folder } from "$lib/api";

export async function load({ cookies, params }) {
  const token = cookies.get('jwt');

  return await folder(token, params.id)
}