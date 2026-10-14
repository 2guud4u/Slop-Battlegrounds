// The Go side emits this file's sibling JSON (internal/game/fixture_test.go,
// -update-fixture). The type assertions below fail tsc when the server's wire
// schema drifts from the client types — every key the client reads must exist
// in the fixture's shape.
import type fixture from "./state.json";
import type { State, FightView, FighterView, PlayerView, ForgeTemplate, Card } from "../components/types";

type Fixture = typeof fixture;
type AssertNoMissing<ClientShape, FixtureShape> =
  Exclude<keyof ClientShape, keyof FixtureShape> extends never ? true : never;

type _state = AssertNoMissing<State, Fixture>;
type _fight = AssertNoMissing<FightView, NonNullable<Fixture["fight"]>>;
type _fighter = AssertNoMissing<FighterView, NonNullable<Fixture["fight"]>["a"]>;
type _player = AssertNoMissing<PlayerView, Fixture["players"][number]>;
type _template = AssertNoMissing<ForgeTemplate, Fixture["templates"][number]>;
type _card = AssertNoMissing<Card, Fixture["hand"][number]>;

export type WireCheck = [_state, _fight, _fighter, _player, _template, _card];
