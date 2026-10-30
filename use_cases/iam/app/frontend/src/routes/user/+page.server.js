import { userTopLevel } from "$lib/api";

export async function load({ locals }) {
  return await userTopLevel(locals.token)
}