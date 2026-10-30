import { API_URL } from '$env/static/private';

export async function login(username) {
  const response = await fetch(API_URL + '/auth/login', {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify({
      username
    })
  })

  const json = await response.json()
  return json.jwt
}

export async function userTopLevel(token) {
  const response = await fetch(API_URL + `/users`, {
    method: "GET",
    headers: {
      "Authorization": `Bearer ${token}`
    },
  })

  return await response.json()
}

export async function folder(token, id) {
  const response = await fetch(API_URL + `/folders/${id}`, {
    method: "GET",
    headers: {
      "Authorization": `Bearer ${token}`
    },
  })

  return await response.json()
}

export async function file(token, id) {
  const response = await fetch(API_URL + `/files/${id}`, {
    method: "GET",
    headers: {
      "Authorization": `Bearer ${token}`
    },
  })

  return {
    content: await response.text(),
  }
}