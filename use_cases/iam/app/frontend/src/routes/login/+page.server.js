import { redirect } from "@sveltejs/kit";
import { login } from "$lib/api";
import jwt from 'jsonwebtoken';
import { JWT_SECRET } from '$env/static/private';

export const load = async (event) => {
  // If user already has JWT, redirect to their profile page,
  // otherwise, keep them on the login page.
  const cookieJWT = event.cookies.get("jwt");
  if (!cookieJWT) {
    return
  }

  const claims = jwt.verify(cookieJWT, JWT_SECRET)
  throw redirect(301, `/users/${claims.user_id}`);
};

export const actions = {
  default: async ({ cookies, request }) => {
    const formData = await request.formData();
    const username = formData.get("username");

    const cookieJWT = await login(username);
    cookies.set("jwt", cookieJWT, {
      path: "/",
      maxAge: 60 * 60,
      httpOnly: false,
    });

    // Redirect user to their profile page.
    jwt.verify(cookieJWT, JWT_SECRET)

    throw redirect(301, "/user")
  },
};
