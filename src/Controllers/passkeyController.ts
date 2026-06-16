import type { SessionUser } from '../Models/types';
import { api } from './apiController';

type PasskeyCredentialDescriptorJSON = {
  type: PublicKeyCredentialType;
  id: string;
  transports?: AuthenticatorTransport[];
};

type PasskeyCreationOptionsJSON = Omit<PublicKeyCredentialCreationOptions, 'challenge' | 'user' | 'excludeCredentials'> & {
  challenge: string;
  user: Omit<PublicKeyCredentialUserEntity, 'id'> & { id: string };
  excludeCredentials?: PasskeyCredentialDescriptorJSON[];
};

type PasskeyRequestOptionsJSON = Omit<PublicKeyCredentialRequestOptions, 'challenge' | 'allowCredentials'> & {
  challenge: string;
  allowCredentials?: PasskeyCredentialDescriptorJSON[];
};

type PasskeyOptions<T> = {
  publicKey: T;
};

type AttestationCredentialJSON = {
  id: string;
  rawId: string;
  type: PublicKeyCredentialType;
  response: {
    attestationObject: string;
    clientDataJSON: string;
  };
};

type AssertionCredentialJSON = {
  id: string;
  rawId: string;
  type: PublicKeyCredentialType;
  response: {
    authenticatorData: string;
    clientDataJSON: string;
    signature: string;
    userHandle?: string;
  };
};

export function supportsPasskeys() {
  return window.isSecureContext && 'PublicKeyCredential' in window && Boolean(navigator.credentials);
}

export async function registerPasskey() {
  const options = await api<PasskeyOptions<PasskeyCreationOptionsJSON>>('/api/passkey/register/options', { method: 'POST' });
  const credential = await navigator.credentials.create({
    publicKey: creationOptionsFromJSON(options.publicKey),
  });
  if (!isPublicKeyCredential(credential) || !('attestationObject' in credential.response)) {
    throw new Error('Nao foi possivel criar a chave de acesso.');
  }
  await api<{ ok: boolean }>('/api/passkey/register', {
    method: 'POST',
    body: JSON.stringify(attestationToJSON(credential)),
  });
}

export async function loginWithSavedPasskey(signal?: AbortSignal) {
  let options: PasskeyOptions<PasskeyRequestOptionsJSON>;
  try {
    options = await api<PasskeyOptions<PasskeyRequestOptionsJSON>>('/api/passkey/login/options', { method: 'POST' });
  } catch (err) {
    if (err instanceof Error && err.message.includes('chave de acesso')) return null;
    throw err;
  }

  const credential = await navigator.credentials.get({
    publicKey: requestOptionsFromJSON(options.publicKey),
    mediation: 'conditional',
    signal,
  } as CredentialRequestOptions);

  if (!credential) return null;
  if (!isPublicKeyCredential(credential) || !('authenticatorData' in credential.response)) {
    throw new Error('Chave de acesso invalida.');
  }

  return api<SessionUser>('/api/passkey/login', {
    method: 'POST',
    body: JSON.stringify(assertionToJSON(credential)),
  });
}

export function isPasskeyAbort(err: unknown) {
  return err instanceof DOMException && (err.name === 'AbortError' || err.name === 'NotAllowedError');
}

function creationOptionsFromJSON(options: PasskeyCreationOptionsJSON): PublicKeyCredentialCreationOptions {
  return {
    ...options,
    challenge: base64URLToBuffer(options.challenge),
    user: {
      ...options.user,
      id: base64URLToBuffer(options.user.id),
    },
    excludeCredentials: options.excludeCredentials?.map(descriptorFromJSON),
  };
}

function requestOptionsFromJSON(options: PasskeyRequestOptionsJSON): PublicKeyCredentialRequestOptions {
  return {
    ...options,
    challenge: base64URLToBuffer(options.challenge),
    allowCredentials: options.allowCredentials?.map(descriptorFromJSON),
  };
}

function descriptorFromJSON(descriptor: PasskeyCredentialDescriptorJSON): PublicKeyCredentialDescriptor {
  return {
    ...descriptor,
    id: base64URLToBuffer(descriptor.id),
  };
}

function attestationToJSON(credential: PublicKeyCredential): AttestationCredentialJSON {
  const response = credential.response as AuthenticatorAttestationResponse;
  return {
    id: credential.id,
    rawId: bufferToBase64URL(credential.rawId),
    type: 'public-key',
    response: {
      attestationObject: bufferToBase64URL(response.attestationObject),
      clientDataJSON: bufferToBase64URL(response.clientDataJSON),
    },
  };
}

function assertionToJSON(credential: PublicKeyCredential): AssertionCredentialJSON {
  const response = credential.response as AuthenticatorAssertionResponse;
  return {
    id: credential.id,
    rawId: bufferToBase64URL(credential.rawId),
    type: 'public-key',
    response: {
      authenticatorData: bufferToBase64URL(response.authenticatorData),
      clientDataJSON: bufferToBase64URL(response.clientDataJSON),
      signature: bufferToBase64URL(response.signature),
      ...(response.userHandle ? { userHandle: bufferToBase64URL(response.userHandle) } : {}),
    },
  };
}

function isPublicKeyCredential(credential: Credential | null): credential is PublicKeyCredential {
  if (!credential) return false;
  return credential.type === 'public-key' && 'rawId' in credential;
}

function base64URLToBuffer(value: string) {
  const base64 = value.replace(/-/g, '+').replace(/_/g, '/');
  const padded = base64.padEnd(Math.ceil(base64.length / 4) * 4, '=');
  const binary = window.atob(padded);
  const bytes = new Uint8Array(binary.length);
  for (let idx = 0; idx < binary.length; idx += 1) {
    bytes[idx] = binary.charCodeAt(idx);
  }
  return bytes.buffer;
}

function bufferToBase64URL(buffer: ArrayBuffer) {
  const bytes = new Uint8Array(buffer);
  let binary = '';
  bytes.forEach((byte) => {
    binary += String.fromCharCode(byte);
  });
  return window.btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '');
}
