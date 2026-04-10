import { create } from "zustand";
import type { Course, PracticeResult, RoleplaySession, Theater, User } from "./types";

type AppState = {
  user?: User;
  theater?: Theater;
  theaters: Theater[];
  courses: Course[];
  roleplay?: RoleplaySession;
  result?: PracticeResult;
  loading: boolean;
  setUser: (user?: User) => void;
  setTheater: (theater?: Theater) => void;
  setTheaters: (theaters: Theater[]) => void;
  setCourses: (courses: Course[]) => void;
  setRoleplay: (roleplay?: RoleplaySession) => void;
  setResult: (result?: PracticeResult) => void;
  setLoading: (loading: boolean) => void;
};

export const useAppStore = create<AppState>((set) => ({
  loading: false,
  theaters: [],
  courses: [],
  setUser: (user) => set({ user }),
  setTheater: (theater) => set({ theater }),
  setTheaters: (theaters) => set({ theaters }),
  setCourses: (courses) => set({ courses }),
  setRoleplay: (roleplay) => set({ roleplay }),
  setResult: (result) => set({ result }),
  setLoading: (loading) => set({ loading })
}));
