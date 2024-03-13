import { file } from "$lib/api";

export async function load({ locals, params }) {
  const token = locals.token;
 
  return await file(token, params.id)
}