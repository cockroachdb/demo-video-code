import { redirect } from "@sveltejs/kit";
import 'jsonwebtoken';

export const load = async (event) => {
  event.cookies.delete('jwt', {
    path: "/",
  })

  throw redirect(301, "/login");
};