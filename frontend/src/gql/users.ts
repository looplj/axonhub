export const USERS_QUERY = `
  query Users($first: Int, $after: Cursor) {
    users(first: $first, after: $after) {
      edges {
        node {
          id
          createdAt
          updatedAt
          email
          firstName
          lastName
          isOwner
          scopes
          roles {
            edges {
              node {
                id
                name
              }
            }
          }
        }
      }
      pageInfo {
        hasNextPage
        hasPreviousPage
        startCursor
        endCursor
      }
    }
  }
`;

export const USER_QUERY = `
  query User($id: ID!) {
    user(id: $id) {
      id
      createdAt
      updatedAt
      email
      firstName
      lastName
      isOwner
      scopes
      roles {
        edges {
          node {
            id
            name
          }
        }
      }
    }
  }
`;

export const CREATE_USER_MUTATION = `
  mutation CreateUser($input: CreateUserInput!) {
    createUser(input: $input) {
      id
      createdAt
      updatedAt
      email
      firstName
      lastName
      isOwner
      scopes
      roles {
        edges {
          node {
            id
            name
          }
        }
      }
    }
  }
`;

export const UPDATE_USER_MUTATION = `
  mutation UpdateUser($id: ID!, $input: UpdateUserInput!) {
    updateUser(id: $id, input: $input) {
      id
      createdAt
      updatedAt
      email
      firstName
      lastName
      isOwner
      scopes
      roles {
        edges {
          node {
            id
            name
          }
        }
      }
    }
  }
`;

export const DELETE_USER_MUTATION = `
  mutation DeleteUser($id: ID!) {
    deleteUser(id: $id)
  }
`;