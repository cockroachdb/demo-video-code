export async function handle({ event, resolve }) {
  const token = event.cookies.get('jwt');
  event.locals.token = token;
 
  const response = await resolve(event);
  return response;
}